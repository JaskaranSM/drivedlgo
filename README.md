# Drive-Dl-Go
A Minimal Google Drive Downloader Written in Go

# Features
- Concurrent File Downloads
- Folder Download Support
- Clones Folder on Local as it was structured on G-Drive
- Progress bar with ETA and Speeds
- Custom Path for Downloading file/folder into
- Download from G-Drive Shareable link support 
- Database for storing credentials and token

# Documentation

## Getting Google OAuth API credential file

- Visit the Google Cloud Console
- Go to the OAuth Consent tab, fill it, and save.
- Go to the Credentials tab and click Create Credentials -> OAuth Client ID
- Choose Other/desktop and Create.
- Use the download button to download your credentials.

## Adding Credentials in application's database

`
drivedl set <path_to_credentials.json>
`

## Using the tool

- Usage can be found in --help
`
drivedl --help
`

## Note:-
First time run after set command will authorize the credentials and generate token. 

