package main

/*
TODOS:
1) get token from cache on certain calls and call delete entities on CAAS
2) update tokens from cache on refresh calls
*/
import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yh742/cgw/pkg/cgw"
)

func main() {
	// find the config file path
	cfgPath := flag.String("cfg", "/etc/cgw/config.yaml", "Path for configuration file")
	logLevel := flag.Int("loglevel", 1, "Set log level (trace=-1, debug=0, info=1, warn=2, error=3)")
	flag.Parse()

	// set log time format
	log.Logger = log.With().Caller().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.Level(*logLevel))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	gw, err := cgw.NewCAASGateway(*cfgPath, cgw.RedisStore{}, nil)
	if err != nil {
		cgw.ErrorLog("can't create new gateway, %s", err)
		return
	}
	go gw.StartServer()
	<-sigs
	gw.StopSignal <- struct{}{}
}
