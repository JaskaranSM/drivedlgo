package main

import (
	"drivedlgo/db"
	"drivedlgo/drive"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"

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
	GD.SetAbusiveFileDownload(c.Bool("acknowledge-abuse"))
	cus_path, err := db.GetDLDirDb()
	if err == nil {
		if c.String("path") == "." {
			path.Join(cus_path, c.String("path"))
		} else {
			cus_path = c.String("path")
		}
	} else {
		cus_path = c.String("path")
	}
	GD.Download(fileId, cus_path)
	return nil
}

func setCredsCallback(c *cli.Context) error {
	arg := c.Args().Get(0)
	if arg == "" {
		return errors.New("Provide a proper credentials.json file path.")
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

func setDLDirCallback(c *cli.Context) error {
	arg := c.Args().Get(0)
	if arg == "" {
		return errors.New("Provide a proper download directory path.")
	}
	fmt.Printf("Detected download directory path: %s\n", arg)
	_, err := db.GetDLDirDb()
	if err == nil {
		db.RemoveDLDirDb()
	}
	_, err = db.AddDLDirDb(arg)
	return err
}

func rmDLDirCallback(c *cli.Context) error {
	_, err := db.GetDLDirDb()
	if err != nil {
		fmt.Println("DB doesnt contain default directory path, try --help.")
	} else {
		_, err = db.RemoveDLDirDb()
		if err == nil {
			fmt.Println("Default directory removed successfully, now application will download in current working directory.")
		} else {
			fmt.Println("Error while removing default directory: ", err.Error())
		}
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
		&cli.BoolFlag{
			Name:  "acknowledge-abuse",
			Usage: "Enable downloading of files marked as abusive by google drive.",
		},
	}
	app := cli.NewApp()
	app.Name = "Google Drive Downloader"
	app.Usage = "A minimal Google Drive Downloader written in Go."
	app.UsageText = fmt.Sprintf("%s [global options] [arguments...]", os.Args[0])
	app.Authors = []cli.Author{
		{Name: "JaskaranSM"},
	}
	app.Action = downloadCallback
	app.Flags = dlFlags
	app.Commands = []cli.Command{
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
		{
			Name:   "setdldir",
			Usage:  "set default download directory",
			Action: setDLDirCallback,
		},
		{
			Name:   "rmdldir",
			Usage:  "remove default download directory and set the application to download in current folder.",
			Action: rmDLDirCallback,
		},
	}
	app.Version = "1.5"
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
