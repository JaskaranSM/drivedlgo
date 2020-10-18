package drive

import (
	"drive-dl-go/db"
	"drive-dl-go/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/cheggaaa/pb"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

var wg sync.WaitGroup

func getProgressBar64(size int64) *pb.ProgressBar {
	bar := pb.New64(size)
	bar.ShowSpeed = true
	bar.SetUnits(pb.U_BYTES)
	return bar
}

type GoogleDriveClient struct {
	GDRIVE_DIR_MIMETYPE string
	TokenFile           string
	CredentialFile      string
	DriveSrv            *drive.Service
	Concurrent          int
	Tasks               int
	isRunning           bool
	ProgressBars        []*pb.ProgressBar
}

func (G *GoogleDriveClient) Init() {
	G.GDRIVE_DIR_MIMETYPE = "application/vnd.google-apps.folder"
	G.TokenFile = "token.json"
	G.CredentialFile = "credentials.json"
	G.Concurrent = 2
	G.Tasks = 0
}

func (G *GoogleDriveClient) SetConcurrency(count int) {
	fmt.Printf("Using Concurrency: %d\n", count)
	G.Concurrent = count
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

func (G *GoogleDriveClient) AddProgressBar(bar *pb.ProgressBar) {
	G.ProgressBars = append(G.ProgressBars, bar)
}

func (G *GoogleDriveClient) SpinProgressBars() {
	if !G.isRunning && len(G.ProgressBars) != 0 {
		G.isRunning = true
		pool, err := pb.StartPool(G.ProgressBars...)
		if err != nil {
			log.Fatal("[PoolError]: %v\n", err)
		}
		wg.Wait()
		pool.Stop()
		G.Clean()
	}
}

func (G *GoogleDriveClient) Clean() {
	G.ProgressBars = nil
	G.Tasks = 0
	G.isRunning = false
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
			log.Printf("%s already downloaded.\n", file.Name)
			return
		}
		if bytesDled != 0 {
			log.Printf("Resuming %s at ByteOffset %d\n", file.Name, bytesDled)
		}
		bar := getProgressBar64(file.Size)
		bar.Start()
		go G.DownloadFile(file, absPath, bar, bytesDled)
		wg.Add(1)
	}
	G.SpinProgressBars()
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
				log.Printf("%s already downloaded.\n", file.Name)
				continue
			}
			if bytesDled != 0 {
				log.Printf("Resuming %s at ByteOffset %d\n", file.Name, bytesDled)
			}
			if G.Concurrent == G.Tasks {
				G.SpinProgressBars()
			}
			bar := getProgressBar64(file.Size - bytesDled)
			G.AddProgressBar(bar)
			go G.DownloadFile(file, absPath, bar, bytesDled)
			wg.Add(1)
			G.Tasks += 1
		}
	}
}

func (G *GoogleDriveClient) DownloadFile(file *drive.File, localPath string, bar *pb.ProgressBar, startByteIndex int64) bool {
	defer wg.Done()
	defer bar.Finish()
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

	barReader := bar.NewProxyReader(response.Body)
	io.Copy(writer, barReader)
	writer.Close()
	return true
}

func NewDriveClient() *GoogleDriveClient {
	return &GoogleDriveClient{}
}
