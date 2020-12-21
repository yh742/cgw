package ds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Disconnecter interface for all protocols
type Disconnecter interface {
	Disconnect(context.Context, DisconnectRequest) error
}

// MQTTDisconnecter is the handler used to connect to MQTT services
type MQTTDisconnecter struct {
	SuccessCode byte
	ConnOpts    *mqtt.ClientOptions
}

// NewMQTTDisconnecter creates a new disconnector
func NewMQTTDisconnecter(settings MQTTSettings, token string) (Disconnecter, error) {
	var err error
	mAuth := UserPassword{}
	// get username/password based on crednetial type
	if settings.AuthType == CRSBased {
		url, err := URLJoin(settings.CRS.Server, settings.CRS.RegistrationEndpoint)
		if err != nil {
			ErrorLog("unable to join crs registration %s, %s",
				settings.CRS.Server, settings.CRS.RegistrationEndpoint)
			return nil, fmt.Errorf("unable to join crs registration url, %s", err)
		}
		mAuth, err = CRSCredentials(url, settings.CRS.Entity, token, settings.CRS.CfgPath)
		if err != nil {
			return nil, err
		}
	} else if settings.AuthType == FileBased {
		mAuth, err = FileCredentials(settings.AuthFile)
		if err != nil {
			return nil, err
		}
	}
	DebugLog("got %+v", mAuth)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(settings.Server)
	opts.SetUsername(mAuth.user)
	opts.SetPassword(mAuth.password)
	return &MQTTDisconnecter{
		SuccessCode: settings.SuccessCode,
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
	if connReq.ReasonCode > ReasonCode(163) {
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
func (handler *MQTTDisconnecter) Disconnect(ctx context.Context, req DisconnectRequest) error {
	// build clientID string based on entityID
	clientID, err := buildClientID(req)
	if err != nil {
		return fmt.Errorf("error building client ID %s", err)
	}
	DebugLog("clientId created, %s", clientID)
	handler.ConnOpts.SetClientID(clientID)
	handler.ConnOpts.SetCleanSession(true)

	// create client
	client := mqtt.NewClient(handler.ConnOpts)
	token := client.Connect()
	mqttDone := make(chan struct{})
	go func() {
		token.Wait()
		mqttDone <- struct{}{}
	}()
	select {
	case <-mqttDone:
		if token.Error() != nil {
			cToken, ok := token.(*mqtt.ConnectToken)
			if !ok {
				return fmt.Errorf("MQTT response was invalid")
			}
			if cToken.ReturnCode() == handler.SuccessCode {
				return fmt.Errorf("Disconnection was successful %d", cToken.ReturnCode())
			}
			return fmt.Errorf("Unexpected error while trying to connect to mqtt %s, %d", token.Error().Error(), cToken.ReturnCode())
		}
		// connection is created, should really never get here
		client.Disconnect(0)
		return fmt.Errorf("Unexpected state reached")
	case <-ctx.Done():
		return errors.New("timeout occured")
	}
}
