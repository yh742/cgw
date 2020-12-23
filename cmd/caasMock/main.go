package main

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
	logLevel := flag.Int("loglevel", 1, "Set log level (trace=-1, debug=0, info=1, warn=2, error=3)")
	flag.Parse()

	// set log time format
	log.Logger = log.With().Caller().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.Level(*logLevel))

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	stopSignal := make(chan struct{})

	sm := cgw.ServiceMocks{}
	go sm.StartServer("9090", stopSignal)
	<-sigs
	cgw.DebugLog("cancel signal received")
	stopSignal <- struct{}{}
}
