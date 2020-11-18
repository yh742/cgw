package ds

import (
	"encoding/json"
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// DisconnectRequest is the json used for request
type DisconnectRequest struct {
	Entity     string `json:"entity,required"`
	EntityID   string `json:"entityID,required"`
	ReasonCode byte   `json:"reasonCode,required"`
	NextServer string `json:"nextServer,optional"`
}

// DisconnectHandler wraps all the disconnect calls to extract request parameter
func DisconnectHandler(disconnector Disconnecter) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		connReq := &DisconnectRequest{}
		err := json.NewDecoder(req.Body).Decode(connReq)
		if err != nil {
			log.Error().Msgf("error decoding json request %s", err)
			return
		}
		log.Debug().Msgf("received client request, %+v", connReq)
		disconnector.Disconnect(*connReq, w)
	}
}

func startServer(cfgPath string) {
	// read yaml configuration file and create mqtt disconnector
	cfg := Config{}
	cfg.Parse(cfgPath)
	disconnecter, err := NewMQTTDisconnecter(cfg)
	if err != nil {
		log.Fatal().Msgf("unable to create new disconnecter, %s", err)
	}

	// routing
	router := mux.NewRouter()
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
