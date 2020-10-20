package drive

import (
	"drivedlgo/db"
	"drivedlgo/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

var wg sync.WaitGroup

type GoogleDriveClient struct {
	GDRIVE_DIR_MIMETYPE string
	TokenFile           string
	CredentialFile      string
	DriveSrv            *drive.Service
	Progress            *mpb.Progress
	channel             chan int
}

func (G *GoogleDriveClient) Init() {
	G.GDRIVE_DIR_MIMETYPE = "application/vnd.google-apps.folder"
	G.TokenFile = "token.json"
	G.CredentialFile = "credentials.json"
	G.channel = make(chan int, 2)
	G.Progress = mpb.New(mpb.WithWidth(60), mpb.WithRefreshRate(180*time.Millisecond))
}

func (G *GoogleDriveClient) SetConcurrency(count int) {
	fmt.Printf("Using Concurrency: %d\n", count)
	G.channel = make(chan int, count)
}

func (G *GoogleDriveClient) GetProgressBar(size int64, status string) *mpb.Bar {
	bar := G.Progress.AddBar(size, mpb.BarStyle("[=>-|"),
		mpb.PrependDecorators(
			decor.Name(status, decor.WC{W: len(status) + 1, C: decor.DidentRight}),
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 90),
			decor.Name("]"),
			decor.EwmaSpeed(decor.UnitKiB, " % .2f", 60),
		),
	)
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
		request := G.DriveSrv.Files.List().Q("'" + parentId + "' in parents").OrderBy("folder").SupportsAllDrives(true).IncludeTeamDriveItems(true).PageSize(1000).
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
	var status string = ""
	file := G.GetFileMetadata(nodeId)
	fmt.Printf("Name: %s, MimeType: %s\n", file.Name, file.MimeType)
	absPath := path.Join(localPath, file.Name)
	if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
		os.MkdirAll(absPath, 0755)
		G.TraverseNodes(file.Id, absPath)
	} else {
		os.MkdirAll(localPath, 0755)
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
			status = fmt.Sprintf("[Resumed-%d]", bytesDled)
		} else {
			status = fmt.Sprintf("[Downloading]")
		}
		G.channel <- 1
		bar := G.GetProgressBar(file.Size-bytesDled, status)
		go G.DownloadFile(file, absPath, bar, bytesDled)
		wg.Add(1)
		status = ""
	}
	wg.Wait()
}

func (G *GoogleDriveClient) TraverseNodes(nodeId string, localPath string) {
	files := G.GetFilesByParentId(nodeId)
	var status string = ""
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
				status = fmt.Sprintf("[Resumed-%d]", bytesDled)
			} else {
				status = fmt.Sprintf("[Downloading]")
			}
			G.channel <- 1
			bar := G.GetProgressBar(file.Size-bytesDled, status)
			go G.DownloadFile(file, absPath, bar, bytesDled)
			wg.Add(1)
			status = ""
		}
	}
}

func (G *GoogleDriveClient) DownloadFile(file *drive.File, localPath string, bar *mpb.Bar, startByteIndex int64) bool {
	writer, err := os.OpenFile(localPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	writer.Seek(startByteIndex, 0)
	request := G.DriveSrv.Files.Get(file.Id).SupportsAllDrives(true)
	request.Header().Add("Range", fmt.Sprintf("bytes=%d-%d", startByteIndex, file.Size))
	response, err := request.Download()
	if err != nil {
		log.Printf("[API-files:get]: %v", err)
		return false
	}

	if err != nil {
		log.Printf("[FileOpenError]: %v\n", err)
		return false
	}
	proxyReader := bar.ProxyReader(response.Body)
	io.Copy(writer, proxyReader)
	writer.Close()
	proxyReader.Close()
	wg.Done()
	<-G.channel
	return true
}

func NewDriveClient() *GoogleDriveClient {
	return &GoogleDriveClient{}
}
