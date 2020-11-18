package main

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
	Port string       `yaml:"port"`
	MQTT MQTTSettings `yaml:"mqtt"`
	CRS  CRSSettings  `yaml:"crs"`
}

// CRSSettings represents settings for CRS
type CRSSettings struct {
	Entity    string `yaml:"entity"`
	Server    string `yaml:"server"`
	CfgPath   string `yaml:"cfgPath"`
	TokenFile string `yaml:"tokenFile"`
}

// MQTTSettings represents settings for MQTT
type MQTTSettings struct {
	Server      string `yaml:"server"`
	Port        string `yaml:"port"`
	SuccessCode byte   `yaml:"successCode"`
	AuthFile    string `yaml:"authFile"`
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
		strings.TrimSpace(cfg.MQTT.Server) == "" ||
		strings.TrimSpace(cfg.MQTT.Port) == "" {
		log.Error().Msgf("missing one of the required values %s, %s, %s", cfg.Port, cfg.MQTT.Server, cfg.MQTT.Port)
		return errors.New("missing required value")
	}
	// if CRS existings, make sure all fields are populated
	if cfg.CRS != (CRSSettings{}) {
		if strings.TrimSpace(cfg.CRS.Entity) == "" ||
			strings.TrimSpace(cfg.CRS.Server) == "" ||
			strings.TrimSpace(cfg.CRS.CfgPath) == "" ||
			strings.TrimSpace(cfg.CRS.TokenFile) == "" {
			log.Error().Msgf("missing one of the CRS required values %s, %s, %s, %s",
				cfg.CRS.Entity, cfg.CRS.Server, cfg.CRS.CfgPath, cfg.CRS.TokenFile)
			return errors.New("missing required CRS value")
		}
	}
	return nil
}
