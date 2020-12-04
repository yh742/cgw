package ds

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

// deleteEntityID looks up the token based on entity ID and delete its from CAAS
func deleteEntityID(entityID string, reasonCode ReasonCode) error {
	// check if reasoncode requires a upstream delete call to CAAS
	if _, ok := upstreamReasonCodes[reasonCode]; !ok {
		return nil
	}
	data, err := HTTPRequest("GET", getTokenEndpoint, map[string]string{"entityid": entityID}, nil, http.StatusOK)
	if err != nil {
		log.Error().Msgf("unable to make request, %v", err)
		return errors.New("unable to make request")
	}
	tokenStruct := &struct {
		Token string `json:"token,required"`
	}{}
	err = json.Unmarshal(data, tokenStruct)
	if err != nil {
		log.Error().Msgf("unable to unmarshal data, '%s', %v", data, err)
		return errors.New("unable to marshal data")
	}
	deleteReq := DeleteEntityRequest{
		Entity: "veh",
		EntityTokenPair: EntityTokenPair{
			EntityID: entityID,
			Token:    tokenStruct.Token,
		},
	}
	jBytes, err := json.Marshal(deleteReq)
	if err != nil {
		log.Error().Msgf("unable to marshal delete entity request, %v", err)
		return errors.New("unable to marshal delete entity request")
	}

	data, err = HTTPRequest("POST", deleteEntityIDEndpoint, nil, bytes.NewBuffer(jBytes), http.StatusOK)
	if err != nil {
		log.Error().Msgf("unable to make delete entity id request, %v", err)
		return errors.New("unable to make delete entity id request")
	}
	return nil
}

// DisconnectHandler wraps all the disconnect calls to extract request parameter
func DisconnectHandler(disconnector Disconnecter) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		connReq := &DisconnectRequest{}
		err := json.NewDecoder(req.Body).Decode(connReq)
		if err != nil {
			log.Error().Msgf("error decoding json request %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Debug().Msgf("received client request, %+v", connReq)
		// perform lookup of token based on entityID
		err = deleteEntityID(connReq.EntityID, connReq.ReasonCode)
		if err != nil {
			log.Error().Msgf("error looking up token based on json request %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		disconnector.Disconnect(*connReq, w)
	}
}

// RefreshHandler is used to handle refresh calls, proxies request to CRS cache
func RefreshHandler(w http.ResponseWriter, req *http.Request) {
	log.Debug().Msgf("received client request, %+v", req)
	_, err := HTTPRequest("POST", updateTokenEndpoint, nil, req.Body, http.StatusOK)
	if err != nil {
		log.Error().Msgf("error sending request to refresh endpoint, %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}
