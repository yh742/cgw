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
	upstreamReasonCodes   map[ReasonCode]bool
	kv                    RedisStore
	disconnecter          Disconnecter
	mecID                 string
	debugSettings         DebugSettings
	requestLog            []interface{}
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
	caasGW.debugSettings = cfg.DebugSettings
	if caasGW.debugSettings.DebugLog {
		caasGW.requestLog = make([]interface{}, 0)
	}

	// stop signal
	caasGW.StopSignal = make(chan struct{})
	return caasGW, nil
}

// AppendLog adds log to log history
func (cgw *CAASGateway) AppendLog(key string, data interface{}) {
	// append to request log
	if cgw.debugSettings.DebugLog {
		cgw.requestLog = append(cgw.requestLog, map[string]interface{}{
			key: data,
		})
	}
}

// ClearLogs erases logs
func (cgw *CAASGateway) ClearLogs() {
	if cgw.debugSettings.DebugLog {
		cgw.requestLog = make([]interface{}, 0)
	}
}

// GetLogs retrieves logs from log history
func (cgw *CAASGateway) GetLogs() []interface{} {
	// append to request log
	return cgw.requestLog
}

// GetToken reads token
func (cgw *CAASGateway) GetToken() string {
	return cgw.token
}

// SetToken writes token
func (cgw *CAASGateway) SetToken(token string) {
	cgw.token = token
}

// GetMEC reads MEC
func (cgw *CAASGateway) GetMEC() string {
	return cgw.mecID
}

// SetMEC writes to the MEC field
func (cgw *CAASGateway) SetMEC(mec string) {
	cgw.mecID = mec
}

// StartServer serves the ds service
func (cgw *CAASGateway) StartServer() {
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// define routing scheme
	router := mux.NewRouter()
	createTokenHandle := createNewTokenHandler(cgw.kv, cgw.caasCreateURL, cgw.GetMEC, cgw.GetToken)
	disconnectHandle := disconnectHandler(cgw.disconnecter, cgw.kv,
		cgw.caasDeleteEntityIDURL, cgw.upstreamReasonCodes, cgw.GetMEC, cgw.GetToken)

	router.Handle("/cgw/v1/token",
		http.TimeoutHandler(
			jsonDecodeHandler(EntityTokenReq, redisLockHandler(cgw.kv, cgw.handlerTO, createTokenHandle), cgw.AppendLog),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")

	router.Handle("/cgw/v1/token/validate",
		http.TimeoutHandler(
			jsonDecodeHandler(EntityTokenReq,
				redisLockHandler(cgw.kv, cgw.handlerTO,
					validateTokenHandler(cgw.kv)), cgw.AppendLog),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")

	router.Handle("/cgw/v1/token/refresh",
		http.TimeoutHandler(
			jsonDecodeHandler(EntityTokenReq,
				redisLockHandler(cgw.kv, cgw.handlerTO,
					refreshTokenHandler(cgw.kv)), cgw.AppendLog),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")

	router.Handle("/cgw/v1/disconnect",
		http.TimeoutHandler(
			jsonDecodeHandler(DisconnectionReq,
				redisLockHandler(cgw.kv, cgw.handlerTO, disconnectHandle), cgw.AppendLog),
			cgw.handlerTO, "Timed out processing request")).Methods("POST")

	if cgw.debugSettings != (DebugSettings{}) {
		flushURL := cgw.debugSettings.FlushEndpoint
		tokenURL := cgw.debugSettings.TokenEndpoint
		mecURL := cgw.debugSettings.MECEndpoint
		reqURL := cgw.debugSettings.ReqLogEndpoint

		if !IsEmpty(flushURL) {
			DebugLog("debug flush endpoint is enabled, %s", flushURL)
			router.Handle(flushURL, http.TimeoutHandler(flushHandler(cgw.kv),
				cgw.handlerTO, "Timed out processing request")).Methods("POST")
		}

		if !IsEmpty(tokenURL) {
			DebugLog("debug token endpoint is enabled, %s", tokenURL)
			router.Handle(tokenURL, http.TimeoutHandler(setTokenHandler(cgw.SetToken),
				cgw.handlerTO, "Timed out processing request")).Methods("GET")
		}

		if !IsEmpty(mecURL) {
			DebugLog("debug mec endpoint is enabled, %s", mecURL)
			router.Handle(mecURL, http.TimeoutHandler(setMECHandler(cgw.SetMEC),
				cgw.handlerTO, "Timed out processing request")).Methods("GET")
		}

		if !IsEmpty(reqURL) {
			DebugLog("debug disconnection info endpoint is enabled, %s", reqURL)
			router.Handle(reqURL, http.TimeoutHandler(getReqLogHandler(cgw.GetLogs),
				cgw.handlerTO, "Timed out processing request")).Methods("GET")
			router.Handle(reqURL, http.TimeoutHandler(delReqLogHandler(cgw.ClearLogs),
				cgw.handlerTO, "Timed out processing request")).Methods("DELETE")
		}
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
