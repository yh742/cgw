package ds

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

// AuthType represent the type of mqtt authentication used
type AuthType byte

// Different auth types
const (
	NoAuth    AuthType = 0x00
	FileBased AuthType = 0x01
	CRSBased  AuthType = 0x02
)

// UserPassword represents username password pair
type UserPassword struct {
	user     string
	password string
}

// CRSCredentials creates auth scheme based on CRS entityID
func CRSCredentials(url string, entity string, token string, crsCfg string) (UserPassword, error) {
	// read configuration file
	cFile, err := os.Open(crsCfg)
	if err != nil {
		log.Error().Msg("can't opne the crs config file")
		return UserPassword{}, err
	}
	defer cFile.Close()
	fBytes, err := ioutil.ReadAll(cFile)
	if err != nil {
		ErrorLog("cannot read the crs config file %s", err)
		return UserPassword{}, err
	}

	// create request client to get entity ID from CRS
	data, err := HTTPRequest(context.Background(), "POST", url,
		map[string]string{"Authorization": "Bearer " + token}, nil, bytes.NewBuffer(fBytes))
	if err != nil {
		ErrorLog("CRS request failed, %v", err)
		return UserPassword{}, errors.New("crs request failed")
	}
	if data.status != http.StatusCreated {
		ErrorLog("CRS request failed with status code: %d", data.status)
		return UserPassword{}, errors.New("CRS request failed with status code")
	}
	idStruct := &struct{ ID string }{}
	err = json.Unmarshal(data.body, idStruct)

	DebugLog("obtained %v", idStruct)
	if err != nil {
		ErrorLog("unable to unmarshal data, %s, %d, %v", data.body, data.status, err)
		return UserPassword{}, errors.New("unable to marshal data")
	}
	DebugLog("obtained entity id, %s", idStruct.ID)

	// assign user and password
	return UserPassword{
		user:     entity + "-" + idStruct.ID,
		password: token,
	}, nil
}

// FileCredentials reads user/password off a file
func FileCredentials(filePath string) (UserPassword, error) {
	if IsEmpty(filePath) {
		log.Error().Msg("file path to parse auth is empty, skipping")
		return UserPassword{}, errors.New("file path to auth file is empty")
	}
	var file *os.File
	var err error
	if file, err = os.Open(filePath); err != nil {
		DebugLog("opening file error: %s", err)
		return UserPassword{}, err
	}
	defer file.Close()

	// create a scanner to read file
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	if ok := scanner.Scan(); !ok {
		ErrorLog("couldn't read user from %s", filePath)
		return UserPassword{}, errors.New("can't read user from auth file")
	}
	user := scanner.Text()
	if ok := scanner.Scan(); !ok {
		ErrorLog("couldn't read password from %s", filePath)
		return UserPassword{}, errors.New("can't read password from auth file")
	}
	psw := scanner.Text()
	return UserPassword{
		user,
		psw,
	}, nil
}
