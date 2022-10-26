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

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var wg sync.WaitGroup

const MAX_NAME_CHARACTERS int = 17
const MAX_RETRIES int = 5

type GoogleDriveClient struct {
	GDRIVE_DIR_MIMETYPE string
	TokenFile           string
	CredentialFile      string
	DriveSrv            *drive.Service
	Progress            *mpb.Progress
	abuse               bool
	channel             chan int
	silent              bool
}

func (G *GoogleDriveClient) Init() {
	G.GDRIVE_DIR_MIMETYPE = "application/vnd.google-apps.folder"
	G.TokenFile = "token.json"
	G.CredentialFile = "credentials.json"
	G.channel = make(chan int, 2)
	G.Progress = mpb.New(mpb.WithWidth(60), mpb.WithRefreshRate(180*time.Millisecond))
	log.SetOutput(G.Progress)
}

func (G *GoogleDriveClient) SetAbusiveFileDownload(abuse bool) {
	fmt.Printf("Acknowledge-Abuse: %t\n", abuse)
	G.abuse = abuse
}

func (G *GoogleDriveClient) SetConcurrency(count int) {
	fmt.Printf("Using Concurrency: %d\n", count)
	G.channel = make(chan int, count)
}

func (G *GoogleDriveClient) SetSilent(silent bool) {
	if silent {
		fmt.Println("Not printing live progress")
		G.Progress.Shutdown()
	}
	G.silent = silent
}

func (G *GoogleDriveClient) GetProgressBar(filename string, size int64) *mpb.Bar {
	var bar *mpb.Bar
	if len(filename) > MAX_NAME_CHARACTERS {
		marquee := customdec.NewChangeNameDecor(filename, MAX_NAME_CHARACTERS)
		bar = G.Progress.AddBar(size,
			mpb.PrependDecorators(
				decor.Name("[ "),
				marquee.MarqueeText(),
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
		bar = G.Progress.AddBar(size,
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

func (G *GoogleDriveClient) getClient(dbPath string, config *oauth2.Config) *http.Client {
	tokBytes, err := db.GetTokenDb(dbPath)
	var tok *oauth2.Token
	if err != nil {
		tok = G.getTokenFromWeb(config)
		db.AddTokenDb(dbPath, utils.OauthTokenToBytes(tok))
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

func (G *GoogleDriveClient) Authorize(dbPath string) {
	credsJsonBytes, err := db.GetCredentialsDb(dbPath)
	if err != nil {
		log.Fatalf("Unable to Get Credentials from Db, make sure to use set command: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(credsJsonBytes, drive.DriveScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := G.getClient(dbPath, config)
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

func (G *GoogleDriveClient) Download(nodeId string, localPath string, outputPath string) {
	file := G.GetFileMetadata(nodeId)
	fmt.Printf("Name: %s, MimeType: %s\n", file.Name, file.MimeType)
	if outputPath == "" {
		outputPath = utils.CleanupFilename(file.Name)
	}
	absPath := path.Join(localPath, outputPath)
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
		G.channel <- 1
		wg.Add(1)
		go G.HandleDownloadFile(file, absPath)
	}
	wg.Wait()
}

func (G *GoogleDriveClient) TraverseNodes(nodeId string, localPath string) {
	files := G.GetFilesByParentId(nodeId)
	for _, file := range files {
		absPath := path.Join(localPath, utils.CleanupFilename(file.Name))
		if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
			err := os.MkdirAll(absPath, 0755)
			if err != nil {
				log.Printf("[DirectoryCreationError]: %v\n", err)
				continue
			}
			G.TraverseNodes(file.Id, absPath)
		} else {
			G.channel <- 1
			wg.Add(1)
			go G.HandleDownloadFile(file, absPath)
		}
	}
}

func (G *GoogleDriveClient) HandleDownloadFile(file *drive.File, absPath string) {
	defer func() {
		wg.Done()
		<-G.channel
	}()

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
	G.DownloadFile(file, absPath, bytesDled, 1)
}

func (G *GoogleDriveClient) DownloadFile(file *drive.File, localPath string, startByteIndex int64, retry int) bool {
	cleanup := func() {
	}
	writer, err := os.OpenFile(localPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	defer writer.Close()
	if err != nil {
		log.Printf("[FileOpenError]: %v\n", err)
		cleanup()
		return false
	}
	writer.Seek(startByteIndex, 0)
	request := G.DriveSrv.Files.Get(file.Id).AcknowledgeAbuse(G.abuse).SupportsAllDrives(true)
	request.Header().Add("Range", fmt.Sprintf("bytes=%d-%d", startByteIndex, file.Size))
	response, err := request.Download()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate") || response.StatusCode >= 500 && retry <= 5 {
			time.Sleep(5 * time.Second)
			return G.DownloadFile(file, localPath, startByteIndex, retry+1)
		}
		log.Printf("[API-files:get]: (%s) %v\n", file.Id, err)
		cleanup()
		return false
	}
	var (
		proxyReader io.ReadCloser
		bar         *mpb.Bar
	)
	if G.silent {
		proxyReader = response.Body
	} else {
		bar = G.GetProgressBar(file.Name, file.Size-startByteIndex)
		proxyReader = bar.ProxyReader(response.Body)
	}
	defer proxyReader.Close()
	_, err = io.Copy(writer, proxyReader)
	if err != nil {
		pos, posErr := writer.Seek(0, os.SEEK_CUR)
		if posErr != nil {
			log.Printf("Error while getting current file offset, %v\n", err)
		} else if retry <= MAX_RETRIES {
			if !G.silent {
				bar.Abort(true)
			}
			time.Sleep(time.Duration(int64(retry)*2) * time.Second)
			return G.DownloadFile(file, localPath, pos, retry+1)
		} else {
			log.Printf("Error while copying stream, %v\n", err)
		}
	}
	cleanup()
	return true
}

func NewDriveClient() *GoogleDriveClient {
	return &GoogleDriveClient{}
}
