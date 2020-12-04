package ds

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// Global endpoints
var getTokenEndpoint string
var updateTokenEndpoint string
var deleteEntityIDEndpoint string
var crsRegistrationEndpoint string

// Upstream to CAAS reason codes
var upstreamReasonCodes map[ReasonCode]bool

// setEndpoints sets the endpoints that DS will use to call CAAS/CRS
func setEndpoints(cfg Config) error {
	var err error
	getTokenEndpoint, err = URLJoin(cfg.CRS.Server, cfg.CRS.GetToken)
	if err != nil {
		log.Error().Msgf("unable to join get token url %s, %s", cfg.CRS.Server, cfg.CRS.GetToken)
		return fmt.Errorf("unable to join get token url, %s", err)
	}
	updateTokenEndpoint, err = URLJoin(cfg.CRS.Server, cfg.CRS.UpdateToken)
	if err != nil {
		log.Error().Msgf("unable to join update token url %s, %s", cfg.CRS.Server, cfg.CRS.UpdateToken)
		return fmt.Errorf("unable to update token url, %s", err)
	}
	deleteEntityIDEndpoint, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.DeleteEntityID)
	if err != nil {
		log.Error().Msgf("unable to join delete entity id url %s, %s", cfg.CAAS.Server, cfg.CAAS.DeleteEntityID)
		return fmt.Errorf("unable to delete entity id url, %s", err)
	}
	crsRegistrationEndpoint, err = URLJoin(cfg.CRS.Server, cfg.CRS.Registration)
	if err != nil {
		log.Error().Msgf("unable to join delete entity id url %s, %s", cfg.CRS.Server, cfg.CRS.Registration)
		return fmt.Errorf("unable to delete entity id url, %s", err)
	}
	upstreamReasonCodes = map[ReasonCode]bool{}
	for _, rc := range cfg.UpstreamReasonCode {
		upstreamReasonCodes[rc] = true
	}
	return nil
}

// StartServer serves the ds service
func StartServer(cfgPath string) {
	// read yaml configuration file and create mqtt disconnector
	cfg := Config{}
	err := cfg.Parse(cfgPath)
	if err != nil {
		log.Fatal().Msgf("unable to parse config file, %s", err)
	}
	// set endpoints
	err = setEndpoints(cfg)
	if err != nil {
		log.Fatal().Msgf("unable to parse config file, %s", err)
	}
	// create disconnecter
	disconnecter, err := NewMQTTDisconnecter(cfg)
	if err != nil {
		log.Fatal().Msgf("unable to create new disconnecter, %s", err)
	}
	// routing
	router := mux.NewRouter()
	router.HandleFunc("/ds/v1/refresh", RefreshHandler).Methods("POST")
	router.HandleFunc("/ds/v1/disconnect", DisconnectHandler(disconnecter)).Methods("POST")
	log.Fatal().Msgf("%s", http.ListenAndServe("0.0.0.0:"+cfg.Port, router))
}
