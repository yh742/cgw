package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func main() {
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// define routing scheme
	router := mux.NewRouter()
	// router.HandleFunc("/ds/v1/token", http.TimeoutHandler()).Methods("POST")
	// router.HandleFunc("/ds/v1/token/validate").Methods("POST")
	// router.HandleFunc("/ds/v1/token/refresh", RefreshHandler).Methods("POST")
	// router.HandleFunc("/ds/v1/disconnect", DisconnectHandler(disconnecter)).Methods("POST")

	srv := &http.Server{
		Addr:    "0.0.0.0:9090",
		Handler: router,
	}

	go func() {
		defer httpServerExitDone.Done()
		// start server
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Msgf("ListenAndServe() failed: %+V", err)
		}
	}()

	// setting up signal capturing, waiting for SIGINT (pkill -2)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	// graceful shut down
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		panic(err)
	}
	httpServerExitDone.Wait()
	log.Info().Msg("finished shutting down server")
}
