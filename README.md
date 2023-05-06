# Drive-Dl-Go

A Minimal Google Drive Downloader Written in Go

## Features

- Concurrent file downloads
- Folder download support
- Clones folder on local as it was structured on google drive
- Progress bar with eta and speeds
- Custom path for downloading files/folder into
- Download from google drive shareable link support
- Database for storing credentials and token
- Resuming on partially downloaded files
- Skipping existing files

## Table of contents

- [Installation](#installation)
  - [Windows](#windows)
    - [Via GitHub](#via-github)
    - [Via Scoop (recommended)](#via-scoop-recommended)
  - [Linux](#linux)
    - [Via Wget](#via-wget)
    - [Via cURL](#via-curl)
  - [macOS](#macos)
- [Documentation](#documentation)
  - [Create Google OAuth client ID credentials](#create-google-oauth-client-id-credentials)
    - [Enable the API](#enable-the-api)
    - [Getting credentials for a desktop application](#getting-credentials-for-a-desktop-application)
    - [Detailed Guides](#detailed-guides)
- [Configuration](#configuration)
- [Usage](#usage)
- [Examples](#examples)

## Installation

### Windows

#### Via GitHub

- Download drivedlgo here: [drivedlgo binaries](https://github.com/JaskaranSM/drivedlgo/releases)
- Extract the `drivedlgo.exe` from the archive.
- add `drivedlgo` to [Environment Variables](https://www.architectryan.com/2018/03/17/add-to-the-path-on-windows-10/)
- run `drivedlgo --help` (run the binary from terminal, double clicking it won't work)

#### Via Scoop (recommended)

[Scoop](https://scoop.sh) users can download and install the latest Drive-Dl-Go release by installing the `drivedlgo` package:

```powershell
scoop bucket add missing-apps https://github.com/semisoft0072/scoop-apps
scoop install drivedlgo
```

To update Drive-Dl-Go using Scoop, run the following:

```powershell
scoop update drivedlgo
```

If you have any issues when installing/updating the package, please search for
or report the same on the [issues
page](https://github.com/semisoft0072/scoop-apps/issues) of Scoop missing-apps bucket repository.

### Linux

#### Via Wget

```bash
wget https://github.com/JaskaranSM/drivedlgo/releases/download/1.6.6/drivedlgo_1.6.6_Linux_x86_64.gz && gzip -d drivedlgo_1.6.6_Linux_x86_64.gz && mv drivedlgo_1.6.6_Linux_x86_64 drivedlgo && chmod +x drivedlgo && sudo mv drivedlgo /usr/bin && drivedlgo --help
```

#### Via cURL

```bash
curl -LO https://github.com/JaskaranSM/drivedlgo/releases/download/1.6.6/drivedlgo_1.6.6_Linux_x86_64.gz && gzip -d drivedlgo_1.6.6_Linux_x86_64.gz && mv drivedlgo_1.6.6_Linux_x86_64 drivedlgo && chmod +x drivedlgo && sudo mv drivedlgo /usr/bin && drivedlgo --help
```

### macOS

Currently no support for macos.

## Documentation

### Create Google OAuth client ID credentials

#### Enable the API

Before using Google APIs, you need to turn them on in a Google Cloud project. You can turn on one or more APIs in a single Google Cloud project.

In the [Google Cloud console](https://console.cloud.google.com/flows/enableapi?apiid=drive.googleapis.com), enable the Google Drive API.

#### Getting credentials for a desktop application

To authenticate as an end user and access user data in your app, you need to create one or more OAuth 2.0 Client IDs. A client ID is used to identify a single app to Google's OAuth servers. If your app runs on multiple platforms, you must create a separate client ID for each platform.

1. In the Google Cloud console, go to Menu ≡ > APIs & Services > [Credentials](https://console.cloud.google.com/apis/credentials).
2. Click Create Credentials > OAuth client ID.
3. Click Application type > Desktop app.
4. In the Name field, type a name for the credential. This name is only shown in the Google Cloud console.
5. Click Create. The OAuth client created screen appears, showing your new Client ID and Client secret.
6. Click OK. and Use the download button to download your credentials.

#### Detailed Guides

- [Google Workspace](https://developers.google.com/workspace/guides/get-started)
- [glotlabs](https://github.com/glotlabs/gdrive/blob/main/docs/create_google_api_credentials.md)
- [rclone](https://rclone.org/drive/#making-your-own-client-id)

## Configuration

- Add your own account client ID our [rclone](https://github.com/semisoft0072/drivedlgo/blob/README.md/rclone.json) credentials.json file to database by running

```terminal
drivedlgo set "/PATH/credentials.json"
```

- After set command will authorize the credentials and generate token by running

```terminal
drivedlgo https://drive.google.com/
```

## Usage

```terminal
❯ drivedlgo --help
NAME:
   Google Drive Downloader - A minimal Google Drive Downloader written in Go.

USAGE:
   C:\Users\user\scoop\apps\drivedlgo\current\drivedlgo.exe [global options] [arguments...]

VERSION:
   1.6

AUTHOR:
   JaskaranSM

COMMANDS:
   set       add credentials.json file to database
   rm        remove credentials from database
   setsa     add service account to database
   rmsa      remove service account from database
   setdldir  set default download directory
   rmdldir   remove default download directory and set the application to download in current folder.
   help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --path value         Folder path to store the download. (default: ".")
   --output value       File/folder name of the download.
   --db-path value      File path to store the database. (default: "C:\\Users\\username\\AppData\\Roaming\\drivedlgo/drivedl-go-db")
   --conn value         Number of Concurrent File Downloads. (default: 2)
   --acknowledge-abuse  Enable downloading of files marked as abusive by google drive.
   --usesa              Use service accounts instead of OAuth.
   --port value         Port for the OAuth web server. (default: 8096)
   --help, -h           show help
   --version, -v        print the version
```

## Examples

Download file/folder

`drivedlgo Link`

Set default download directory

`drivedlgo setdldir "Path"`

Remove default download directory

`drivedlgo rmdldir`

Download multiple Files (default: 2)

`drivedlgo "Link" --conn 10`

Download files marked as abusive by google drive

`drivedlgo "Link" --acknowledge-abuse`
