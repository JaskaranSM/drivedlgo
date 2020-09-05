package main

import (
	"drive-dl-go/db"
	"drive-dl-go/drive"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"runtime"

	"github.com/urfave/cli"
)

const DRIVE_LINK_REGEX string = `https://drive\.google\.com/(drive)?/?u?/?\d?/?(mobile)?/?(file)?(folders)?/?d?/([-\w]+)[?+]?/?(w+)?`

func getFileIdByLink(link string) string {
	match := regexp.MustCompile(DRIVE_LINK_REGEX)
	matches := match.FindStringSubmatch(link)
	if len(matches) >= 2 {
		return matches[len(matches)-2]
	}
	urlParsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	values := urlParsed.Query()
	if len(values) == 0 {
		return ""
	}
	for i, j := range values {
		if i == "id" {
			return j[0]
		}
	}
	return ""
}

func downloadCallback(c *cli.Context) error {
	arg := c.Args().Get(0)
	if arg == "" {
		return errors.New(fmt.Sprintf("Required argument <fileid/link> is missing. \nUsage: %s\nFor more info: %s --help ", c.App.UsageText, os.Args[0]))
	}
	fileId := getFileIdByLink(arg)
	if fileId == "" {
		fileId = arg
	}
	fmt.Printf("Detected File-Id: %s\n", fileId)
	GD := drive.NewDriveClient()
	GD.Init()
	GD.Authorize()
	GD.SetConcurrency(c.Int("conn"))
	GD.Download(fileId, c.String("path"))
	return nil
}

func setCredsCallback(c *cli.Context) error {
	arg := c.Args().Get(0)
	if arg == "" {
		return errors.New("Provide a Proper credentials.json file path.")
	}
	fmt.Printf("Detected credentials.json Path: %s\n", arg)
	if !db.IsCredentialsInDb() {
		if db.IsTokenInDb() {
			db.RemoveTokenDb()
		}
		db.AddCredentialsDb(arg)
		fmt.Printf("%s added in database.\n", arg)
	} else {
		fmt.Println("A credentials file already exists in databse, use rm command to remove it first.")
	}
	return nil
}

func rmCredsCallback(c *cli.Context) error {
	if db.IsCredentialsInDb() {
		db.RemoveCredentialsDb()
		db.RemoveTokenDb()
		fmt.Println("credentials removed from database successfully.")
	} else {
		fmt.Println("Database doesnt contain any credentials.")
	}
	return nil
}

func main() {
	dlFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  "path",
			Usage: "Folder path to store the download.",
			Value: ".",
		},
		&cli.IntFlag{
			Name:  "conn",
			Usage: "Number of Concurrent File Downloads.",
			Value: 2,
		},
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	app := cli.NewApp()
	app.Name = "Google Drive Downloader"
	app.Usage = "A minimal Google Drive Downloader written in Go."
	app.UsageText = fmt.Sprintf("%s [global options] [arguments...]", os.Args[0])
	app.Authors = []*cli.Author{
		{Name: "JaskaranSM"},
	}
	app.Action = downloadCallback
	app.Flags = dlFlags
	app.Commands = []*cli.Command{
		{
			Name:   "set",
			Usage:  "add credentials.json file to database",
			Action: setCredsCallback,
		},
		{
			Name:   "rm",
			Usage:  "remove credentials from database",
			Action: rmCredsCallback,
		},
	}
	app.Version = "1.1"
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
