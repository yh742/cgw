package cgw

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// CAASGateway is the gateway to caas
type CAASGateway struct {
	port                  string
	readTO                time.Duration
	writeTO               time.Duration
	handlerTO             time.Duration
	maxHeaderBytes        int
	token                 string
	caasCreateURL         string
	caasDeleteEntityIDURL string
	flushEndpoint         string
	upstreamReasonCodes   map[ReasonCode]bool
	kv                    RedisStore
	disconnecter          Disconnecter
	mecID                 string
	StopSignal            chan struct{}
}

// NewCAASGateway creates a new gateway instance
func NewCAASGateway(cfgPath string, redis RedisStore, disconnecter Disconnecter) (CAASGateway, error) {
	// read yaml configuration file and create mqtt disconnector
	cfg, err := NewConfig(cfgPath)
	if err != nil {
		ErrorLog("unable to parse config file %s", cfgPath)
		return CAASGateway{}, fmt.Errorf("unable to parse config file, %s", err)
	}

	caasGW := CAASGateway{
		port:           cfg.Port,
		readTO:         time.Duration(cfg.ReadTimeout) * time.Millisecond,
		writeTO:        time.Duration(cfg.WriteTimeout) * time.Millisecond,
		handlerTO:      time.Duration(cfg.HandlerTimeout) * time.Millisecond,
		maxHeaderBytes: cfg.MaxHeaderBytes,
		mecID:          cfg.MECID,
	}

	// set endpoints
	caasGW.caasCreateURL, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.CreateEndpoint)
	if err != nil {
		ErrorLog("unable to join get caas validate url %s, %s",
			cfg.CAAS.Server, cfg.CAAS.CreateEndpoint)
		return CAASGateway{}, fmt.Errorf("unable to caas validate url, %s", err)
	}
	caasGW.caasDeleteEntityIDURL, err = URLJoin(cfg.CAAS.Server, cfg.CAAS.DeleteEndpoint)
	if err != nil {
		ErrorLog("unable to join caas delete entityid url %s, %s",
			cfg.CAAS.Server, cfg.CAAS.DeleteEndpoint)
		return CAASGateway{}, fmt.Errorf("unable to join caas delete entityid url, %s", err)
	}

	// read token file
	tFile, err := os.Open(cfg.TokenFile)
	if err != nil {
		msg := fmt.Sprintf("can't open the token file, %s", cfg.TokenFile)
		ErrorLog(msg)
		return CAASGateway{}, errors.New(msg)
	}
	defer tFile.Close()
	tBytes, err := ioutil.ReadAll(tFile)
	if err != nil {
		msg := fmt.Sprintf("can't read the token file, %s", cfg.TokenFile)
		ErrorLog(msg)
		return CAASGateway{}, errors.New(msg)
	}
	if IsEmpty(string(tBytes)) {
		msg := "token is empty"
		ErrorLog(msg)
		return CAASGateway{}, errors.New(msg)
	}
	caasGW.token = string(tBytes)

	// set upstream reasoncodes
	caasGW.upstreamReasonCodes = map[ReasonCode]bool{}
	for _, rc := range cfg.UpstreamReasonCode {
		caasGW.upstreamReasonCodes[rc] = true
	}

	// assign disconnecter and redis to gateway, if not passed in
	if disconnecter == nil {
		caasGW.disconnecter, err = NewMQTTDisconnecter(cfg.MQTT, caasGW.token)
		if err != nil {
			msg := fmt.Sprintf("can't create disconnecter, %s", err)
			ErrorLog(msg)
			return CAASGateway{}, errors.New(msg)
		}
	} else {
		caasGW.disconnecter = disconnecter
	}

	if redis == (RedisStore{}) {
		caasGW.kv, err = NewRedisStore(cfg.Redis)
		if err != nil {
			msg := fmt.Sprintf("can't create redis store, %s", err)
			ErrorLog(msg)
			return CAASGateway{}, errors.New(msg)
		}
	} else {
		caasGW.kv = redis
	}

	// stop signal
	caasGW.StopSignal = make(chan struct{})

	// create flush endpoint if not empty
	if !IsEmpty(cfg.FlushEndpoint) {
		caasGW.flushEndpoint = cfg.FlushEndpoint
	}
	return caasGW, nil
}

// StartServer serves the ds service
func (cgw *CAASGateway) StartServer() {
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// define routing scheme
	router := mux.NewRouter()
	router.Handle("/cgw/v1/token",
		http.TimeoutHandler(
			jsonDecodeHandler(EntityTokenReq,
				redisLockHandler(cgw.kv, cgw.handlerTO,
					createNewTokenHandler(cgw.kv, cgw.caasCreateURL, cgw.mecID, cgw.token))),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")
	router.Handle("/cgw/v1/token/validate",
		http.TimeoutHandler(
			jsonDecodeHandler(EntityTokenReq,
				redisLockHandler(cgw.kv, cgw.handlerTO,
					validateTokenHandler(cgw.kv))),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")
	router.Handle("/cgw/v1/token/refresh",
		http.TimeoutHandler(
			jsonDecodeHandler(EntityTokenReq,
				redisLockHandler(cgw.kv, cgw.handlerTO,
					refreshTokenHandler(cgw.kv))),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")
	router.Handle("/cgw/v1/disconnect",
		http.TimeoutHandler(
			jsonDecodeHandler(DisconnectionReq,
				redisLockHandler(cgw.kv, cgw.handlerTO,
					disconnectHandler(
						cgw.disconnecter, cgw.kv, cgw.caasDeleteEntityIDURL, cgw.upstreamReasonCodes, cgw.token))),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")

	if !IsEmpty(cgw.flushEndpoint) {
		DebugLog("debug flush endpoint is enabled, %s", cgw.flushEndpoint)
		router.Handle(cgw.flushEndpoint, http.TimeoutHandler(flushHandler(cgw.kv),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")
	}

	// create server instance
	srv := &http.Server{
		Addr:           ":" + cgw.port,
		Handler:        router,
		ReadTimeout:    cgw.readTO,
		WriteTimeout:   cgw.writeTO,
		MaxHeaderBytes: cgw.maxHeaderBytes,
	}

	go func() {
		defer httpServerExitDone.Done()
		// start server
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Msgf("ListenAndServe() failed: %+v", err)
		}
	}()

	<-cgw.StopSignal

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		panic(err)
	}
	httpServerExitDone.Wait()
	log.Info().Msg("finished shutting down server")
}
