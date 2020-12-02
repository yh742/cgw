package main

/*
TODOS:
1) get token from cache on certain calls and call delete entities on CAAS
2) update tokens from cache on refresh calls
*/
import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// For accessing cache
var getTokenEndpoint string
var updateTokenEndpoint string

// For deleting entity IDs from CAAS
var deleteEntityIDEndpoint string
var downstreamReasonCodes map[ReasonCode]bool

// DisconnectRequest is the json used for request
type DisconnectRequest struct {
	Entity     string     `json:"entity,required"`
	EntityID   string     `json:"entityid,required"`
	ReasonCode ReasonCode `json:"reasonCode,required"`
	NextServer string     `json:"nextServer,optional"`
}

// RefreshRequest is the json used for request
type RefreshRequest struct {
	EntityID string `json:"entityid,required"`
	Token    string `json:"token,required"`
}

// DeleteEntityRequest is the json used for deleting entity requests
type DeleteEntityRequest struct {
	RefreshRequest
	Entity string `json:"entity,required"`
}

// DeleteEntityID looks up the token based on entity ID and delete its from CAAS
func DeleteEntityID(entityID string, reasonCode ReasonCode) error {
	// check if reasoncode requires a upstream delete call to CAAS
	if _, ok := downstreamReasonCodes[reasonCode]; !ok {
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
		RefreshRequest: RefreshRequest{
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
		log.Error().Msgf("unable to make request, %v", err)
		return errors.New("unable to make request")
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
		err = DeleteEntityID(connReq.EntityID, connReq.ReasonCode)
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

func startServer(cfgPath string) {
	// read yaml configuration file and create mqtt disconnector
	cfg := Config{}
	err := cfg.Parse(cfgPath)
	if err != nil {
		log.Fatal().Msgf("unable to parse config file, %s", err)
	}

	// setup global values
	getTokenEndpoint, err = URLJoin(cfg.CRS.Server, cfg.CRS.GetToken)
	if err != nil {
		log.Fatal().Msgf("unable to join get url %s, %s", cfg.CRS.Server, cfg.CRS.GetToken)
	}
	updateTokenEndpoint, err = URLJoin(cfg.CRS.Server, cfg.CRS.UpdateToken)
	if err != nil {
		log.Fatal().Msgf("unable to join update url %s, %s", cfg.CRS.Server, cfg.CRS.UpdateToken)
	}
	deleteEntityIDEndpoint, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.DeleteEntityID)
	if err != nil {
		log.Fatal().Msgf("unable to join delete url %s, %s", cfg.CAAS.Server, cfg.CAAS.DeleteEntityID)
	}
	for _, rc := range cfg.UpstreamReasonCode {
		downstreamReasonCodes[rc] = true
	}

	// create disconnecter
	disconnecter, err := NewMQTTDisconnecter(cfg)

	if err != nil {
		log.Fatal().Msgf("unable to create new disconnecter, %s", err)
	}

	// routing
	router := mux.NewRouter()
	router.HandleFunc("/refresh", RefreshHandler).Methods("POST")
	router.HandleFunc("/disconnect", DisconnectHandler(disconnecter)).Methods("POST")
	log.Fatal().Msgf("%s", http.ListenAndServe("0.0.0.0:"+cfg.Port, router))
}

func main() {
	// find the config file path
	cfgPath := flag.String("cfg", "/etc/ds/config.yaml", "Path for configuration file")
	logLevel := flag.Int("loglevel", 1, "Set log level (trace=-1, debug=0, info=1, warn=2, error=3)")
	flag.Parse()

	// set log time format
	log.Logger = log.With().Caller().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.Level(*logLevel))

	// start server instance
	startServer(*cfgPath)
}
