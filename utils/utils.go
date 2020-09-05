package utils

import (
	"bytes"
	"encoding/gob"

	"github.com/OpenPeeDeeP/xdg"
	"golang.org/x/oauth2"
)

const APP_NAME = "drive-dl-go"

func GetDbBasePath() string {
	xdg_helper := xdg.New("", APP_NAME)
	return xdg_helper.ConfigHome()
}

func BytesToOauthToken(data []byte) *oauth2.Token {
	token := &oauth2.Token{}
	dec := gob.NewDecoder(bytes.NewReader(data))
	dec.Decode(token)
	return token
}

func OauthTokenToBytes(token *oauth2.Token) []byte {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	enc.Encode(token)
	newBytes := buffer.Bytes()
	return newBytes
}
