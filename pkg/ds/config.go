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
	MECID              string        `yaml:"mecID"`
	MaxHeaderBytes     int           `yaml:"maxHeaderBytes"`
	ReadTimeout        int           `yaml:"readTimeout"`
	WriteTimeout       int           `yaml:"writeTimeout"`
	HandlerTimeout     int           `yaml:"handlerTimeout"`
	Port               string        `yaml:"port"`
	UpstreamReasonCode []ReasonCode  `yaml:"upstreamReasonCode"`
	MQTT               MQTTSettings  `yaml:"mqtt"`
	CAAS               CAASSettings  `yaml:"caas"`
	Redis              RedisSettings `yaml:"redis"`
}

// MQTTSettings represents settings for MQTT
type MQTTSettings struct {
	Server      string      `yaml:"server"`
	SuccessCode byte        `yaml:"successCode"`
	AuthType    AuthType    `yaml:"authType"`
	AuthFile    string      `yaml:"authFile"`
	CRS         CRSSettings `yaml:"crs"`
}

// CRSSettings represents settings for CRS
type CRSSettings struct {
	Entity               string `yaml:"entity"`
	Server               string `yaml:"server"`
	CfgPath              string `yaml:"cfgPath"`
	TokenFile            string `yaml:"tokenFile"`
	RegistrationEndpoint string `yaml:"registrationEndpoint"`
}

// CAASSettings represents settings for CAAS
type CAASSettings struct {
	Server           string `yaml:"server"`
	CreateEndpoint   string `yaml:"createEndpoint"`
	DeleteEndpoint   string `yaml:"deleteEndpoint"`
	ValidateEndpoint string `yaml:"validateEndpoint"`
}

// RedisSettings represents settings for Redis
type RedisSettings struct {
	Server   string `yaml:"server"`
	AuthFile string `yaml:"authFile"`
	DBIndex  int    `yaml:"DBIndex"`
}

// Parse the file provided in path
func (cfg *Config) Parse(path string) error {
	log.Debug().Msgf("parsing file path: %s", path)
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
	if IsEmpty(cfg.Port) || IsEmpty(cfg.MECID) {
		log.Error().Msgf("missing required value %s, %s", cfg.Port, cfg.MECID)
		return errors.New("missing required value")
	}

	// check server timeout value
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 ||
		cfg.MaxHeaderBytes <= 0 || cfg.HandlerTimeout <= 0 {
		log.Error().Msgf("must specify server values;"+
			"readtimeout: %d, writetimeout: %d, maxheaderbytes: %d, handlertimeout: %d",
			cfg.ReadTimeout, cfg.WriteTimeout, cfg.MaxHeaderBytes, cfg.HandlerTimeout)
		return errors.New("invalid server values")
	}

	// make sure all requried servers are populated
	if IsEmpty(cfg.CAAS.Server) || IsEmpty(cfg.MQTT.Server) ||
		IsEmpty(cfg.Redis.Server) {
		log.Error().Msgf("missing one of the required servers; redis: %s, caas: %s, mqtt: %s",
			cfg.Redis.Server, cfg.CAAS.Server, cfg.MQTT.Server)
		return errors.New("missing required server locations")
	}

	// make sure all endpoints are populated
	if IsEmpty(cfg.CAAS.ValidateEndpoint) || IsEmpty(cfg.CAAS.DeleteEndpoint) ||
		IsEmpty(cfg.CAAS.CreateEndpoint) {
		log.Error().Msgf("missing caas endpoints; create: %s, validate: %s, deleteEntityID: %s",
			cfg.CAAS.CreateEndpoint, cfg.CAAS.ValidateEndpoint, cfg.CAAS.DeleteEndpoint)
		return errors.New("missing required endpoint")
	}

	// make sure auth fields are populated
	if cfg.MQTT.AuthType == CRSBased {
		if IsEmpty(cfg.MQTT.CRS.Server) || IsEmpty(cfg.MQTT.CRS.Entity) ||
			IsEmpty(cfg.MQTT.CRS.RegistrationEndpoint) || IsEmpty(cfg.MQTT.CRS.CfgPath) ||
			IsEmpty(cfg.MQTT.CRS.TokenFile) {
			log.Error().Msgf("missing one of the required crs field;"+
				" server: %s, entity: %s, config: %s, token: %s, registration: %s",
				cfg.MQTT.CRS.Server, cfg.MQTT.CRS.Entity, cfg.MQTT.CRS.CfgPath,
				cfg.MQTT.CRS.TokenFile, cfg.MQTT.CRS.RegistrationEndpoint)
			return errors.New("missing required crs auth fields")
		}
	} else if cfg.MQTT.AuthType == FileBased {
		if strings.TrimSpace(cfg.MQTT.AuthFile) == "" {
			log.Error().Msgf("missing mqtt auth file: %s", cfg.MQTT.AuthFile)
			return errors.New("missing required mqtt auth fields")
		}
	}

	// make sure redis auth is populated
	if IsEmpty(cfg.Redis.AuthFile) {
		log.Error().Msgf("missing redis auth file: %s", cfg.Redis.AuthFile)
		return errors.New("missing required redis auth file")
	}
	return nil
}
