package db

import (
	"drivedlgo/utils"
	"io/ioutil"
	"log"
	"path"
	"github.com/prologic/bitcask"
)

const (
	DB_NAME     string = "drivedl-go-db"
	CREDENTIALS string = "credentials"
	TOKEN       string = "token"
)

func getDb() *bitcask.Bitcask {
	db, err := bitcask.Open(path.Join(utils.GetDbBasePath(), DB_NAME))
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func AddCredentialsDb(credsPath string) (bool, error) {
	db := getDb()
	defer db.Close()
	data, err := ioutil.ReadFile(credsPath)
	if err != nil {
		return false, err
	}
	err = db.Put([]byte(CREDENTIALS), data)
	if err != nil {
		return false, err
	}
	return true, nil
}

func AddTokenDb(tok []byte) (bool, error) {
	db := getDb()
	defer db.Close()
	err := db.Put([]byte(TOKEN), tok)
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetCredentialsDb() ([]byte, error) {
	db := getDb()
	defer db.Close()
	data, err := db.Get([]byte(CREDENTIALS))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetTokenDb() ([]byte, error) {
	db := getDb()
	defer db.Close()
	data, err := db.Get([]byte(TOKEN))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func IsCredentialsInDb() bool {
	db := getDb()
	defer db.Close()
	return db.Has([]byte(CREDENTIALS))
}

func IsTokenInDb() bool {
	db := getDb()
	defer db.Close()
	return db.Has([]byte(TOKEN))
}

func RemoveCredentialsDb() (bool, error) {
	db := getDb()
	defer db.Close()
	err := db.Delete([]byte(CREDENTIALS))
	if err != nil {
		return false, err
	}
	return true, nil
}

func RemoveTokenDb() (bool, error) {
	db := getDb()
	defer db.Close()
	err := db.Delete([]byte(TOKEN))
	if err != nil {
		return false, err
	}
	return true, nil
}
