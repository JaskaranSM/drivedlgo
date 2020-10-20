package utils

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/OpenPeeDeeP/xdg"
	"golang.org/x/oauth2"
)

const APP_NAME = "drivedlgo"

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

func GetFileMd5(filePath string) (string, error) {
	fmt.Printf("Calculating Md5Sum of %s\n", filePath)
	var returnMD5String string
	file, err := os.Open(filePath)
	if err != nil {
		return returnMD5String, err
	}
	defer file.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return returnMD5String, err
	}
	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String = hex.EncodeToString(hashInBytes)
	return returnMD5String, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetFileSize(filePath string) (int64, error) {
	file, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	size := file.Size()
	return size, nil
}

func CheckLocalFile(filePath, driveFileHash string) (bool, int64, error) {
	var fileSize int64
	if !fileExists(filePath) {
		return false, 0, nil
	}
	hash, err := GetFileMd5(filePath)
	if err != nil {
		return false, 0, err
	}
	fileSize, err = GetFileSize(filePath)
	if err != nil {
		return false, 0, err
	}
	if hash == driveFileHash {
		return true, fileSize, nil
	}
	return false, fileSize, nil
}

func OauthTokenToBytes(token *oauth2.Token) []byte {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	enc.Encode(token)
	newBytes := buffer.Bytes()
	return newBytes
}

func StringToInt(str string) (int, error) {
	i, err := strconv.Atoi(str)
	if err != nil {
		return 0, err
	}
	return i, nil
}
