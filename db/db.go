package db

import (
	"io/ioutil"
	"log"

	"git.mills.io/prologic/bitcask"
)

const (
	CREDENTIALS string = "credentials"
	TOKEN       string = "token"
	DL_DIR      string = "dl_dir"
)

func getDb(dbPath string) *bitcask.Bitcask {
	db, err := bitcask.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func AddCredentialsDb(dbPath string, credsPath string) (bool, error) {
	db := getDb(dbPath)
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

func AddTokenDb(dbPath string, tok []byte) (bool, error) {
	db := getDb(dbPath)
	defer db.Close()
	err := db.Put([]byte(TOKEN), tok)
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetCredentialsDb(dbPath string) ([]byte, error) {
	db := getDb(dbPath)
	defer db.Close()
	data, err := db.Get([]byte(CREDENTIALS))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetTokenDb(dbPath string) ([]byte, error) {
	db := getDb(dbPath)
	defer db.Close()
	data, err := db.Get([]byte(TOKEN))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func IsCredentialsInDb(dbPath string) bool {
	db := getDb(dbPath)
	defer db.Close()
	return db.Has([]byte(CREDENTIALS))
}

func IsTokenInDb(dbPath string) bool {
	db := getDb(dbPath)
	defer db.Close()
	return db.Has([]byte(TOKEN))
}

func RemoveCredentialsDb(dbPath string) (bool, error) {
	db := getDb(dbPath)
	defer db.Close()
	err := db.Delete([]byte(CREDENTIALS))
	if err != nil {
		return false, err
	}
	return true, nil
}

func RemoveTokenDb(dbPath string) (bool, error) {
	db := getDb(dbPath)
	defer db.Close()
	err := db.Delete([]byte(TOKEN))
	if err != nil {
		return false, err
	}
	return true, nil
}

func AddDLDirDb(dbPath string, dir_path string) (bool, error) {
	db := getDb(dbPath)
	defer db.Close()
	err := db.Put([]byte(DL_DIR), []byte(dir_path))
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetDLDirDb(dbPath string) (string, error) {
	db := getDb(dbPath)
	defer db.Close()
	data, err := db.Get([]byte(DL_DIR))
	if err != nil {
		return ".", err
	}
	return string(data), nil
}

func RemoveDLDirDb(dbPath string) (bool, error) {
	db := getDb(dbPath)
	defer db.Close()
	err := db.Delete([]byte(DL_DIR))
	if err != nil {
		return false, err
	}
	return true, nil
}
