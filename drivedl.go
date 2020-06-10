package main

import (
	"driveHelper"
	"os"
	"log"
	"errors"
	"runtime"
	"github.com/urfave/cli"
)

func downloadCallback(c *cli.Context) error {
	if c.String("fileId") == "" {
		return errors.New("fileId arg is empty, exiting..")
	}
	GD := driveHelper.NewDriveClient()
	GD.Init()
	GD.Authorize()
	GD.SetConcurrency(c.Int("conn"))
	GD.Download(c.String("fileId"),c.String("path"))
	return nil
}

func main(){
	runtime.GOMAXPROCS(runtime.NumCPU())
	app := cli.NewApp()
    app.Name = "Google Drive Downloader"
    app.Usage = "A minimal Google Drive Downloader written in Go."
    app.Authors = []*cli.Author{
        {Name:"JaskaranSM",},
    }
    downloadFlags := []cli.Flag{
        &cli.StringFlag{
            Name:"path",
            Usage:"Folder path to store the download.",
            Value:".",
        },
        &cli.StringFlag{
            Name:"fileId",
            Usage:"Id of Google Drive file/folder.",
        },
        &cli.IntFlag{
            Name:"conn",
            Usage:"Number of Concurrent File Downloads.",
            Value:2,
        },
    }
    app.Commands = cli.Commands{
        {
            Name:"dl",
            Usage:"Download Google Drive file/folder to local.",
            Flags:downloadFlags,
            Action:downloadCallback,
        },
    }
    err := app.Run(os.Args)
    if err != nil {
        log.Fatal(err)
    }
}