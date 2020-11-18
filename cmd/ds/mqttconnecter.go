package ds

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

// Disconnecter interface for all protocols
type Disconnecter interface {
	Disconnect(DisconnectRequest, http.ResponseWriter)
}

// MQTTDisconnecter is the handler used to connect to MQTT services
type MQTTDisconnecter struct {
	SuccessCode byte
	ConnOpts    *mqtt.ClientOptions
}

// NewMQTTDisconnecter creates a new disconnector
func NewMQTTDisconnecter(cfg Config) (Disconnecter, error) {
	var err error
	var mAuth MQTTAuth
	if cfg.CRS != (CRSSettings{}) {
		mAuth, err = CRSCredentials(cfg)
		if err != nil {
			return nil, err
		}
	} else if cfg.MQTT.AuthFile != "" {
		mAuth, err = FileCredentials(cfg)
		if err != nil {
			return nil, err
		}
	} else {
		mAuth = MQTTAuth{}
	}
	log.Debug().Msgf("got %+v", mAuth)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTT.Server + ":" + cfg.MQTT.Port)
	opts.SetUsername(mAuth.user)
	opts.SetPassword(mAuth.password)
	return &MQTTDisconnecter{
		SuccessCode: cfg.MQTT.SuccessCode,
		ConnOpts:    opts,
	}, nil
}

// buildClientID creates a ClientID based on client requests
func buildClientID(connReq DisconnectRequest) (string, error) {
	// (1) check entity type to see if its sw or admin
	if strings.ToLower(connReq.Entity) != "sw" &&
		strings.ToLower(connReq.Entity) != "admin" {
		return "", errors.New("entity type is not supported")
	}
	// (2) check entity ID is not empty
	if strings.TrimSpace(connReq.EntityID) == "" {
		return "", errors.New("entity ID is empty")
	}
	// (3) check reason code, mqtt reason code should be < 163
	if connReq.ReasonCode > byte(163) {
		return "", errors.New("reason code is not valid")
	}
	clientID := []string{
		connReq.Entity,
		connReq.EntityID,
		fmt.Sprintf("%d", connReq.ReasonCode),
	}
	// (4) if next server exists append it
	if strings.TrimSpace(connReq.NextServer) != "" {
		clientID = append(clientID, connReq.NextServer)
	}
	return strings.Join(clientID, "-"), nil
}

// Disconnect initiates a CONNECT call to a MQTT broker
func (handler *MQTTDisconnecter) Disconnect(req DisconnectRequest, w http.ResponseWriter) {
	// build clientID string based on entityID
	clientID, err := buildClientID(req)
	if err != nil {
		log.Error().Msgf("error building client ID %s", err)
		return
	}
	handler.ConnOpts.SetClientID(clientID)
	handler.ConnOpts.SetCleanSession(true)

	// create client
	client := mqtt.NewClient(handler.ConnOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		cToken, ok := token.(*mqtt.ConnectToken)
		if !ok {
			log.Error().Msg("cannot cast mqtt token to connect tokens")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if cToken.ReturnCode() == handler.SuccessCode {
			log.Debug().Msg("disconnection was successful")
			w.WriteHeader(http.StatusOK)
			return
		}
		log.Error().Msgf("unexpected error while trying to connect to mqtt %s, %d", token.Error().Error(), cToken.ReturnCode())
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// connection is created, should really never get here
	client.Disconnect(0)
	log.Error().Msg("mqtt in connected state")
	w.WriteHeader(http.StatusInternalServerError)
}
