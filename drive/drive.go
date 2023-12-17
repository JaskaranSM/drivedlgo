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

	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v8"

	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
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
	numFilesDownloaded  int
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

func (G *GoogleDriveClient) PrepareProgressBar(size int64, dec decor.Decorator) *mpb.Bar {
	return G.Progress.AddBar(size,
		mpb.PrependDecorators(
			decor.Name("[ "),
			dec,
			decor.Name(" ] "),
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.AverageETA(decor.ET_STYLE_GO),
			decor.Name("]"),
			decor.AverageSpeed(decor.SizeB1000(0), " % .2f"),
		),
	)
}

func (G *GoogleDriveClient) GetProgressBar(filename string, size int64) *mpb.Bar {
	if len(filename) > MAX_NAME_CHARACTERS {
		marquee := customdec.NewChangeNameDecor(filename, MAX_NAME_CHARACTERS)
		return G.PrepareProgressBar(size, marquee.MarqueeText())
	}
	return G.PrepareProgressBar(size, decor.Name(filename, decor.WC{W: 5, C: decor.DSyncSpaceR}))
}

func (G *GoogleDriveClient) getClient(dbPath string, config *oauth2.Config, port int) *http.Client {
	tokBytes, err := db.GetTokenDb(dbPath)
	var tok *oauth2.Token
	if err != nil {
		tok = G.getTokenFromWeb(config, port)
		db.AddTokenDb(dbPath, utils.OauthTokenToBytes(tok))
	} else {
		tok = utils.BytesToOauthToken(tokBytes)
	}
	return config.Client(context.Background(), tok)
}

func (G *GoogleDriveClient) getTokenFromHTTP(port int) (string, error) {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	var code string
	var codeReceived chan struct{} = make(chan struct{})
	var err error
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code = r.URL.Query().Get("code")
		_, err = fmt.Fprint(w, "Code received, you can close this browser window now.")
		codeReceived <- struct{}{}
	})
	go func() {
		err = srv.ListenAndServe()
	}()
	if err != nil {
		return code, err
	}
	<-codeReceived
	err = srv.Shutdown(context.Background())
	return code, err
}

func (G *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config, port int) *oauth2.Token {
	config.RedirectURL = fmt.Sprintf("http://localhost:%d", port)
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser: \n%v\n", authURL)
	err := utils.OpenBrowserURL(authURL)
	if err != nil {
		log.Printf("unable to open browser, you have to manually visit the provided link: %v\n", err)
	}
	authCode, err := G.getTokenFromHTTP(port)
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("unable to get token from oauth web: %v\n", err)
	}
	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func (G *GoogleDriveClient) Authorize(dbPath string, useSA bool, port int) {
	var client *http.Client
	if useSA {
		fmt.Println("Authorizing via service-account")
		jwtConfigJsonBytes, err := db.GetJWTConfigDb(dbPath)
		if err != nil {
			log.Fatalf("Unable to Get SA Credentials from Db, make sure to use setsa command: %v", err)
		}
		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.JWTConfigFromJSON(jwtConfigJsonBytes, drive.DriveScope)
		if err != nil {
			log.Fatalf("Unable to parse client secret file to config: %v", err)
		}
		client = config.Client(context.Background())
	} else {
		fmt.Println("Authorizing via google-account")
		credsJsonBytes, err := db.GetCredentialsDb(dbPath)
		if err != nil {
			log.Fatalf("Unable to Get Credentials from Db, make sure to use set command: %v", err)
		}

		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.ConfigFromJSON(credsJsonBytes, drive.DriveScope)
		if err != nil {
			log.Fatalf("Unable to parse client secret file to config: %v", err)
		}
		client = G.getClient(dbPath, config, port)
	}
	srv, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}
	G.DriveSrv = srv
}

func (G *GoogleDriveClient) GetFilesByParentId(parentId string) []*drive.File {
	var files []*drive.File
	pageToken := ""
	for {
		request := G.DriveSrv.Files.List().Q("'" + parentId + "' in parents and trashed=false").OrderBy("name,folder").SupportsAllDrives(true).IncludeTeamDriveItems(true).PageSize(1000).
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
		fmt.Println(err)
		os.Exit(0)
	}
	return file
}

func (G *GoogleDriveClient) Download(nodeId string, localPath string, outputPath string) {
	startTime := time.Now()
	file := G.GetFileMetadata(nodeId)
	if outputPath == "" {
		outputPath = utils.CleanupFilename(file.Name)
	}
	fmt.Printf("%s(%s): %s -> %s/%s\n", color.HiBlueString("Download"), color.GreenString(file.MimeType), color.HiGreenString(file.Id), color.HiYellowString(localPath), color.HiYellowString(outputPath))
	absPath := path.Join(localPath, outputPath)
	if file.MimeType == G.GDRIVE_DIR_MIMETYPE {
		err := os.MkdirAll(absPath, 0755)
		if err != nil {
			fmt.Println("Error while creating directory: ", err.Error())
			return
		}
		files := G.GetFilesByParentId(file.Id)
		if len(files) == 0 {
			fmt.Println("google drive folder is empty.")
		} else {
			G.TraverseNodes(file.Id, absPath)
		}
	} else {
		err := os.MkdirAll(localPath, 0755)
		if err != nil {
			fmt.Println("Error while creating directory: ", err.Error())
			return
		}
		G.channel <- 1
		wg.Add(1)
		go G.HandleDownloadFile(file, absPath)
	}
	wg.Wait()
	G.Progress.Wait()
	fmt.Printf("%s", color.GreenString(fmt.Sprintf("Downloaded %d files in %s.\n", G.numFilesDownloaded, time.Now().Sub(startTime))))
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
		o := fmt.Sprintf("Resuming %s at offset %d\n", file.Name, bytesDled)
		fmt.Printf("%s", color.GreenString(o))
	}
	G.DownloadFile(file, absPath, bytesDled, 1)
}

func (G *GoogleDriveClient) DownloadFile(file *drive.File, localPath string, startByteIndex int64, retry int) bool {
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
		log.Printf("err while requesting download: retrying download: %s: %v\n", file.Name, err)
		if strings.Contains(strings.ToLower(err.Error()), "rate") || response != nil && response.StatusCode >= 500 && retry <= 5 {
			time.Sleep(5 * time.Second)
			return G.DownloadFile(file, localPath, startByteIndex, retry+1)
		}
		log.Printf("[API-files:get]: (%s) %v\n", file.Id, err)
		return false
	}
	bar := G.GetProgressBar(file.Name, file.Size-startByteIndex)
	proxyReader := bar.ProxyReader(response.Body)
	defer proxyReader.Close()
	_, err = io.Copy(writer, proxyReader)
	if err != nil {
		pos, posErr := writer.Seek(0, io.SeekCurrent)
		if posErr != nil {
			log.Printf("Error while getting current file offset, %v\n", err)
			return false
		} else if retry <= MAX_RETRIES {
			log.Printf("err while copying stream: retrying download: %s: %v\n", file.Name, err)
			bar.Abort(true)
			time.Sleep(time.Duration(int64(retry)*2) * time.Second)
			return G.DownloadFile(file, localPath, pos, retry+1)
		} else {
			log.Printf("Error while copying stream, %v\n", err)
		}
	} else {
		G.numFilesDownloaded += 1
	}
	return true
}

func NewDriveClient() *GoogleDriveClient {
	return &GoogleDriveClient{}
}
