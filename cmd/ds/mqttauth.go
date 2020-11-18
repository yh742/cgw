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
	"time"

	"github.com/rs/zerolog/log"
)

// MQTTAuth represents credentials needed to authorize with MQTT
type MQTTAuth struct {
	user     string
	password string
}

// CRSCredentials creates auth scheme based on CRS entityID
func CRSCredentials(cfg Config) (MQTTAuth, error) {
	// read token file
	tFile, err := os.Open(cfg.CRS.TokenFile)
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
	cFile, err := os.Open(cfg.CRS.CfgPath)
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
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("POST", cfg.CRS.Server, bytes.NewBuffer(fBytes))
	if err != nil {
		log.Error().Msgf("post request creation failed, %v", err)
		return MQTTAuth{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Msgf("post request failed, %v", err)
		return MQTTAuth{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		log.Error().Msgf("crs responded with status code, %d", resp.StatusCode)
		return MQTTAuth{}, errors.New("crs responded with failure code")
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().Msgf("unable to read response body, %v", err)
		return MQTTAuth{}, errors.New("can't read response body")
	}
	idStruct := &struct{ ID string }{}
	err = json.Unmarshal(data, idStruct)
	if err != nil {
		log.Error().Msgf("unable to unmarshal data, '%s', %v", data, err)
		return MQTTAuth{}, errors.New("unable to marshal data")
	}
	log.Debug().Msgf("obtained entity id, %s", idStruct.ID)

	// assign user and password
	return MQTTAuth{
		user:     cfg.CRS.Entity + "-" + idStruct.ID,
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
