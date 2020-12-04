package ds

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// Config represents the configuration file
type Config struct {
	Port               string       `yaml:"port"`
	UpstreamReasonCode []ReasonCode `yaml:"upstreamReasonCode"`
	AuthType           AuthType     `yaml:"authType"`
	MQTT               MQTTSettings `yaml:"mqtt"`
	CRS                CRSSettings  `yaml:"crs"`
	CAAS               CAASSettings `yaml:"caas"`
}

// CRSSettings represents settings for CRS
type CRSSettings struct {
	Entity       string `yaml:"entity"`
	Server       string `yaml:"server"`
	CfgPath      string `yaml:"cfgPath"`
	TokenFile    string `yaml:"tokenFile"`
	Registration string `yaml:"registration"`
	GetToken     string `yaml:"getToken"`
	UpdateToken  string `yaml:"updateToken"`
}

// MQTTSettings represents settings for MQTT
type MQTTSettings struct {
	Server      string `yaml:"server"`
	SuccessCode byte   `yaml:"successCode"`
	AuthFile    string `yaml:"authFile"`
}

// CAASSettings represents settings for CAAS
type CAASSettings struct {
	Server         string `yaml:"server"`
	DeleteEntityID string `yaml:"deleteEntityID"`
}

// Parse the file provided in path
func (cfg *Config) Parse(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	bytesRead := bytes.NewReader(data)
	yaml := yaml.NewDecoder(bytesRead)
	err = yaml.Decode(cfg)
	if err != nil {
		return err
	}

	// make sure required fields are populated
	if strings.TrimSpace(cfg.Port) == "" ||
		strings.TrimSpace(cfg.MQTT.Server) == "" {
		log.Error().Msgf("missing one of the required values %s, %s",
			cfg.Port, cfg.MQTT.Server)
		return errors.New("missing required value")
	}

	//make sure all requried endpoints are populated
	if strings.TrimSpace(cfg.CAAS.Server) == "" ||
		strings.TrimSpace(cfg.CRS.Server) == "" {
		log.Error().Msgf("missing one of the required endpoints %s, %s",
			cfg.CAAS.Server, cfg.CRS.Server)
		return errors.New("missing required endpoints")
	}

	// if make sure all fields used for auth is populated
	if cfg.AuthType == CRSBased {
		if strings.TrimSpace(cfg.CRS.Entity) == "" ||
			strings.TrimSpace(cfg.CRS.CfgPath) == "" ||
			strings.TrimSpace(cfg.CRS.TokenFile) == "" {
			log.Error().Msgf("missing one of the CRS required values %s, %s, %s",
				cfg.CRS.Entity, cfg.CRS.CfgPath, cfg.CRS.TokenFile)
			return errors.New("missing required CRS value")
		}
	} else if cfg.AuthType == FileBased {
		if strings.TrimSpace(cfg.MQTT.AuthFile) == "" {
			log.Error().Msg("missing mqtt auth file")
			return errors.New("missing mqtt auth file")
		}
	}
	return nil
}
