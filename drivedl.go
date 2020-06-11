package main

import (
	"driveHelper"
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
		return matches[len(matches) - 2]
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
		return errors.New(fmt.Sprintf("Required argument <fileid/link> is missing. \nUsage: %s\nFor more info: drivedl --help ", c.App.UsageText))
	}
	fileId := getFileIdByLink(arg)
	if fileId == "" {
		fileId = arg
	}
	fmt.Printf("Detected File-Id: %s\n",fileId)
	GD := driveHelper.NewDriveClient()
	GD.Init()
	GD.Authorize()
	GD.SetConcurrency(c.Int("conn"))
	GD.Download(fileId, c.String("path"))
	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	app := cli.NewApp()
	app.Name = "Google Drive Downloader"
	app.Usage = "A minimal Google Drive Downloader written in Go."
	app.UsageText = "drivedl [global options] <fileid/link>"
	app.Authors = []*cli.Author{
		{Name: "JaskaranSM"},
	}
	app.Action = downloadCallback
	app.Version = "1.0"
	app.Flags = []cli.Flag{
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
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
