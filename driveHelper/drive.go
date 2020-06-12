package driveHelper

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	G.Concurrent = count
}

func (G *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	tok, err := G.tokenFromFile(G.TokenFile)
	if err != nil {
		tok = G.getTokenFromWeb(config)
		G.saveToken(G.TokenFile, tok)
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

func (G *GoogleDriveClient) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (G *GoogleDriveClient) saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (G *GoogleDriveClient) Authorize() {
	b, err := ioutil.ReadFile(G.CredentialFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, drive.DriveScope)
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
			Fields("nextPageToken,files(id, name,size, mimeType)")
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
	file, err := G.DriveSrv.Files.Get(fileId).Fields("name,mimeType,size,id").SupportsAllDrives(true).Do()
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
		bar := getProgressBar64(file.Size)
		bar.Start()
		go G.DownloadFile(file, absPath, bar)
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
			if G.Concurrent == G.Tasks {
				G.SpinProgressBars()
			}
			bar := getProgressBar64(file.Size)
			G.AddProgressBar(bar)
			go G.DownloadFile(file, absPath, bar)
			wg.Add(1)
			G.Tasks += 1
		}
	}
}

func (G *GoogleDriveClient) DownloadFile(file *drive.File, localPath string, bar *pb.ProgressBar) bool {
	defer wg.Done()
	defer bar.Finish()
	request := G.DriveSrv.Files.Get(file.Id).SupportsAllDrives(true)
	response, err := request.Download()
	if err != nil {
		log.Printf("[API-files:get]: %v", err)
		return false
	}
	writer, err := os.Create(localPath)
	if err != nil {
		log.Printf("[FileCreationError]: %v\n", err)
		return false
	}
	barReader := bar.NewProxyReader(response.Body)
	io.Copy(writer, barReader)
	return true
}

func NewDriveClient() *GoogleDriveClient {
	return &GoogleDriveClient{}
}
