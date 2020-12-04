package main

/*
TODOS:
1) get token from cache on certain calls and call delete entities on CAAS
2) update tokens from cache on refresh calls
*/
import (
	"flag"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yh742/ds/pkg/ds"
)

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
	ds.StartServer(*cfgPath)
}
