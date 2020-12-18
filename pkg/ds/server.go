package ds

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// CAASGateway is the gateway to caas
type CAASGateway struct {
	port                  string
	readTO                int
	writeTO               int
	handlerTO             int
	maxHeaderBytes        int
	caasCreateURL         string
	caasValidateURL       string
	caasDeleteEntityIDURL string
	upstreamReasonCodes   map[ReasonCode]bool
	kv                    RedisStore
	disconnecter          Disconnecter
	mecID                 string
}

// NewGateway creates a new gateway instance
func NewGateway(cfgPath string) (CAASGateway, error) {
	// read yaml configuration file and create mqtt disconnector
	cfg := Config{}
	err := cfg.Parse(cfgPath)
	if err != nil {
		log.Error().Msgf("unable to parse config file %s", cfgPath)
		return CAASGateway{}, fmt.Errorf("unable to parse config file, %s", err)
	}

	cassGW := CAASGateway{
		port:           cfg.Port,
		readTO:         cfg.ReadTimeout,
		writeTO:        cfg.WriteTimeout,
		handlerTO:      cfg.HandlerTimeout,
		maxHeaderBytes: cfg.MaxHeaderBytes,
		mecID:          cfg.MECID,
	}

	// set endpoints
	cassGW.caasValidateURL, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.ValidateEndpoint)
	if err != nil {
		log.Error().Msgf("unable to join get caas validate url %s, %s",
			cfg.CAAS.Server, cfg.CAAS.ValidateEndpoint)
		return CAASGateway{}, fmt.Errorf("unable to caas validate url, %s", err)
	}
	cassGW.caasValidateURL, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.ValidateEndpoint)
	if err != nil {
		log.Error().Msgf("unable to join get caas validate url %s, %s",
			cfg.CAAS.Server, cfg.CAAS.ValidateEndpoint)
		return CAASGateway{}, fmt.Errorf("unable to caas validate url, %s", err)
	}
	cassGW.caasDeleteEntityIDURL, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.DeleteEndpoint)
	if err != nil {
		log.Error().Msgf("unable to join caas delete entityid url %s, %s",
			cfg.CAAS.Server, cfg.CAAS.DeleteEndpoint)
		return CAASGateway{}, fmt.Errorf("unable to join caas delete entityid url, %s", err)
	}

	// set upstream reasoncodes
	cassGW.upstreamReasonCodes = map[ReasonCode]bool{}
	for _, rc := range cfg.UpstreamReasonCode {
		cassGW.upstreamReasonCodes[rc] = true
	}

	// create disconnecter
	cassGW.disconnecter, err = NewMQTTDisconnecter(cfg)
	if err != nil {
		log.Error().Msgf("unable to create new disconnecter, %s", err)
		return CAASGateway{}, fmt.Errorf("unable to create disconnecter, %s", err)
	}

	// create redis
	cassGW.kv, err = NewRedisStore(cfg.Redis)
	if err != nil {
		log.Error().Msgf("unable to create redis instance, %s", err)
		return CAASGateway{}, fmt.Errorf("unable to create redis instance, %s", err)
	}

	return cassGW, nil
}

// StartServer serves the ds service
func (cgw *CAASGateway) StartServer() {
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// define routing scheme
	router := mux.NewRouter()
	// router.Handle("/ds/v1/token",
	// 	http.TimeoutHandler(createNewToken(cgw.kv, cgw.caasCreateURL, cgw.mecID),
	// 		time.Duration(cgw.handlerTO)*time.Millisecond,
	// 		"Timed out processing request")).Methods("POST")
	// router.HandleFunc("/ds/v1/token/validate",
	// 	http.TimeoutHandler(validateToken(cgw.kv, cgw.caasValidateURL))).Methods("POST")
	// router.HandleFunc("/ds/v1/token/refresh", RefreshHandler).Methods("POST")
	// router.HandleFunc("/ds/v1/disconnect", DisconnectHandler(disconnecter)).Methods("POST")

	srv := &http.Server{
		Addr:           ":" + cgw.port,
		Handler:        router,
		ReadTimeout:    time.Duration(cgw.readTO) * time.Millisecond,
		WriteTimeout:   time.Duration(cgw.writeTO) * time.Millisecond,
		MaxHeaderBytes: cgw.maxHeaderBytes,
	}

	go func() {
		defer httpServerExitDone.Done()
		// start server
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Msgf("ListenAndServe() failed: %+v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		panic(err)
	}
	httpServerExitDone.Wait()
	log.Info().Msg("finished shutting down server")
}
