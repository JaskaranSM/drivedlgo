package drive

import (
	"drivedlgo/customdec"
	"drivedlgo/db"
	"drivedlgo/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

var wg sync.WaitGroup

const MAX_NAME_CHARACTERS int = 17

type GoogleDriveClient struct {
	GDRIVE_DIR_MIMETYPE string
	TokenFile           string
	CredentialFile      string
	DriveSrv            *drive.Service
	Progress            *mpb.Progress
	abuse               bool
	channel             chan int
}

func (G *GoogleDriveClient) Init() {
	G.GDRIVE_DIR_MIMETYPE = "application/vnd.google-apps.folder"
	G.TokenFile = "token.json"
	G.CredentialFile = "credentials.json"
	G.channel = make(chan int, 2)
	G.Progress = mpb.New(mpb.WithWidth(60), mpb.WithRefreshRate(180*time.Millisecond))
}

func (G *GoogleDriveClient) SetAbusiveFileDownload(abuse bool) {
	fmt.Printf("Acknowledge-Abuse: %t\n", abuse)
	G.abuse = abuse
}

func (G *GoogleDriveClient) SetConcurrency(count int) {
	fmt.Printf("Using Concurrency: %d\n", count)
	G.channel = make(chan int, count)
}

func (G *GoogleDriveClient) GetProgressBar(filename string, size int64) *mpb.Bar {
	var bar *mpb.Bar
	if len(filename) > MAX_NAME_CHARACTERS {
		marquee := customdec.NewChangeNameDecor(filename, MAX_NAME_CHARACTERS)
		bar = G.Progress.AddBar(size, mpb.BarStyle("[=>-|"),
			mpb.PrependDecorators(
				decor.Name("[ "),
				marquee.MarqueeText(decor.WC{W: 5, C: decor.DidentRight}),
				decor.Name(" ] "),
				decor.CountersKibiByte("% .2f / % .2f"),
			),
			mpb.AppendDecorators(
				decor.AverageETA(decor.ET_STYLE_GO),
				decor.Name("]"),
				decor.AverageSpeed(decor.UnitKiB, " % .2f"),
			),
		)
	} else {
		bar = G.Progress.AddBar(size, mpb.BarStyle("[=>-|"),
			mpb.PrependDecorators(
				decor.Name("[ "),
				decor.Name(filename, decor.WC{W: 5, C: decor.DidentRight}),
				decor.Name(" ] "),
				decor.CountersKibiByte("% .2f / % .2f"),
			),
			mpb.AppendDecorators(
				decor.AverageETA(decor.ET_STYLE_GO),
				decor.Name("]"),
				decor.AverageSpeed(decor.UnitKiB, " % .2f"),
			),
		)
	}
	return bar
}

func (G *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	tokBytes, err := db.GetTokenDb()
	var tok *oauth2.Token
	if err != nil {
		tok = G.getTokenFromWeb(config)
		db.AddTokenDb(utils.OauthTokenToBytes(tok))
	} else {
		tok = utils.BytesToOauthToken(tokBytes)
	}
	return config.Client(context.Background(), tok)
}

func (G *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func (G *GoogleDriveClient) Authorize() {
	credsJsonBytes, err := db.GetCredentialsDb()
	if err != nil {
		log.Fatalf("Unable to Get Credentials from Db, make sure to use set command: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(credsJsonBytes, drive.DriveScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := G.getClient(config)
	srv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}
	G.DriveSrv = srv
}

func (G *GoogleDriveClient) GetFilesByParentId(parentId string) []*drive.File {
	var files []*drive.File
	pageToken := ""
	for {
		request := G.DriveSrv.Files.List().Q("'" + parentId + "' in parents").OrderBy("name,folder").SupportsAllDrives(true).IncludeTeamDriveItems(true).PageSize(1000).
			Fields("nextPageToken,files(id, name,size, mimeType,md5Checksum)")
		if pageToken != "" {
			request = request.PageToken(pageToken)
		}
		res, err := request.Do()
		if err != nil {
			fmt.Printf("Error : %v", err)
			return files
		}
		files = append(files, res.Files...)
		pageToken = res.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return files
}

func (G *GoogleDriveClient) GetFileMetadata(fileId string) *drive.File {
	file, err := G.DriveSrv.Files.Get(fileId).Fields("name,mimeType,size,id,md5Checksum").SupportsAllDrives(true).Do()
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func (G *GoogleDriveClient) Download(nodeId string, localPath string) {
	file := G.GetFileMetadata(nodeId)
	fmt.Printf("Name: %s, MimeType: %s\n", file.Name, file.MimeType)
	absPath := path.Join(localPath, file.Name)
	if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
		err := os.MkdirAll(absPath, 0755)
		if err != nil {
			log.Println("Error while creating directory: ", err.Error())
			return
		}
		G.TraverseNodes(file.Id, absPath)

	} else {
		err := os.MkdirAll(localPath, 0755)
		if err != nil {
			log.Println("Error while creating directory: ", err.Error())
			return
		}
		exists, bytesDled, err := utils.CheckLocalFile(absPath, file.Md5Checksum)
		if err != nil {
			log.Printf("[FileCheckError]: %v\n", err)
			return
		}
		if exists {
			fmt.Printf("%s already downloaded.\n", file.Name)
			return
		}
		if bytesDled != 0 {
			fmt.Printf("Resuming %s at offset %d\n", file.Name, bytesDled)
		}
		G.channel <- 1
		go G.DownloadFile(file, absPath, bytesDled)
		wg.Add(1)
	}
	wg.Wait()
}

func (G *GoogleDriveClient) TraverseNodes(nodeId string, localPath string) {
	files := G.GetFilesByParentId(nodeId)
	for _, file := range files {
		absPath := path.Join(localPath, file.Name)
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			err := os.MkdirAll(absPath, 0755)
			if err != nil {
				log.Printf("[DirectoryCreationError]: %v\n", err)
				continue
			}
			G.TraverseNodes(file.Id, absPath)
		} else {
			exists, bytesDled, err := utils.CheckLocalFile(absPath, file.Md5Checksum)
			if err != nil {
				log.Printf("[FileCheckError]: %v\n", err)
				continue
			}
			if exists {
				fmt.Printf("%s already downloaded.\n", file.Name)
				continue
			}
			if bytesDled != 0 {
				fmt.Printf("Resuming %s at offset %d\n", file.Name, bytesDled)
			}
			G.channel <- 1
			go G.DownloadFile(file, absPath, bytesDled)
			wg.Add(1)
		}
	}
}

func (G *GoogleDriveClient) DownloadFile(file *drive.File, localPath string, startByteIndex int64) bool {
	defer func() {
		wg.Done()
		<-G.channel
	}()
	writer, err := os.OpenFile(localPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	defer writer.Close()
	if err != nil {
		log.Printf("[FileOpenError]: %v\n", err)
		return false
	}
	writer.Seek(startByteIndex, 0)
	request := G.DriveSrv.Files.Get(file.Id).AcknowledgeAbuse(G.abuse).SupportsAllDrives(true)
	request.Header().Add("Range", fmt.Sprintf("bytes=%d-%d", startByteIndex, file.Size))
	response, err := request.Download()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate") {
			time.Sleep(5 * time.Second)
			return G.DownloadFile(file, localPath, startByteIndex)
		}
		log.Printf("[API-files:get]: (%s) %v\n", file.Id, err)
		return false
	}
	bar := G.GetProgressBar(file.Name, file.Size-startByteIndex)
	proxyReader := bar.ProxyReader(response.Body)
	io.Copy(writer, proxyReader)
	proxyReader.Close()
	return true
}

func NewDriveClient() *GoogleDriveClient {
	return &GoogleDriveClient{}
}
