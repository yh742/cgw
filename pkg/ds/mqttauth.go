package ds

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

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

// MQTTAuth represents credentials needed to authorize with MQTT
type MQTTAuth struct {
	user     string
	password string
}

// CRSCredentials creates auth scheme based on CRS entityID
func CRSCredentials(entity string, tokenFile string, crsCfg string) (MQTTAuth, error) {
	// read token file
	tFile, err := os.Open(tokenFile)
	if err != nil {
		log.Error().Msg("can't open the token config file")
		return MQTTAuth{}, err
	}
	defer tFile.Close()
	tBytes, err := ioutil.ReadAll(tFile)
	if err != nil {
		log.Error().Msgf("cannot read the crs config file %s", err)
		return MQTTAuth{}, err
	}
	// read configuration file
	cFile, err := os.Open(crsCfg)
	if err != nil {
		log.Error().Msg("can't opne the crs config file")
		return MQTTAuth{}, err
	}
	defer cFile.Close()
	fBytes, err := ioutil.ReadAll(cFile)
	if err != nil {
		log.Error().Msgf("cannot read the crs config file %s", err)
		return MQTTAuth{}, err
	}

	// create request client to get entity ID from CRS
	data, err := HTTPRequest("POST", crsRegistrationEndpoint,
		map[string]string{"Authorization": "Bearer " + string(tBytes)}, nil, bytes.NewBuffer(fBytes), http.StatusCreated)
	if err != nil {
		log.Error().Msgf("CRS request failed, %v", err)
		return MQTTAuth{}, errors.New("crs request failed")
	}
	idStruct := &struct{ ID string }{}
	err = json.Unmarshal(data, idStruct)

	log.Debug().Msgf("obtained %v", idStruct)
	if err != nil {
		log.Error().Msgf("unable to unmarshal data, '%s', %v", data, err)
		return MQTTAuth{}, errors.New("unable to marshal data")
	}
	log.Debug().Msgf("obtained entity id, %s", idStruct.ID)

	// assign user and password
	return MQTTAuth{
		user:     entity + "-" + idStruct.ID,
		password: string(tBytes),
	}, nil
}

// FileCredentials reads user/password off a file
func FileCredentials(cfg Config) (MQTTAuth, error) {
	filePath := cfg.MQTT.AuthFile
	if strings.TrimSpace(filePath) == "" {
		log.Error().Msg("file path to parse auth is empty, skipping")
		return MQTTAuth{}, errors.New("file path to auth file is empty")
	}
	var file *os.File
	var err error
	if file, err = os.Open(filePath); err != nil {
		log.Debug().Msgf("opening file error: %s", err)
		return MQTTAuth{}, err
	}
	defer file.Close()

	// create a scanner to read file
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	if ok := scanner.Scan(); !ok {
		log.Error().Msgf("couldn't read user from %s", filePath)
		return MQTTAuth{}, errors.New("can't read user from auth file")
	}
	user := scanner.Text()
	if ok := scanner.Scan(); !ok {
		log.Error().Msgf("couldn't read password from %s", filePath)
		return MQTTAuth{}, errors.New("can't read password from auth file")
	}
	psw := scanner.Text()
	return MQTTAuth{
		user,
		psw,
	}, nil
}
