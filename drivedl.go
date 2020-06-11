package main

import (
	"driveHelper"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/urfave/cli"
)

func downloadCallback(c *cli.Context) error {
	arg := c.Args().Get(0)
	if arg == "" {
		return errors.New(fmt.Sprintf("Required argument <fileid> is missing. \nUsage: %s\nFor more info --help ", c.App.UsageText))
	}
	GD := driveHelper.NewDriveClient()
	GD.Init()
	GD.Authorize()
	GD.SetConcurrency(c.Int("conn"))
	GD.Download(arg, c.String("path"))
	return nil
}

// ./drivedl --conn 4 --path "dls/dl/as" 1Zh5fQ6e6U1ySMA15vKMvDQcIROqlHcpp
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	app := cli.NewApp()
	app.Name = "Google Drive Downloader"
	app.Usage = "A minimal Google Drive Downloader written in Go."
	app.UsageText = "drivedl [global options] <fileid>"
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
