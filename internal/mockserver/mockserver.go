package mockserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// MockServer creates a mock for testing server interactions
type MockServer struct {
	reqHistory      [][]byte
	reqQueryHistory []*url.URL
}

// RunServer starts the server
func (ss *MockServer) RunServer(port string, method string, path string, retVal map[string]string, retCode int, stop <-chan struct{}) {
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    "localhost:" + port,
		Handler: router,
	}
	router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Error().Msg("can't read from request body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ss.reqHistory = append(ss.reqHistory, bytes)
		ss.reqQueryHistory = append(ss.reqQueryHistory, r.URL)
		if retVal != nil {
			mapD := retVal
			mapB, _ := json.Marshal(mapD)
			w.WriteHeader(retCode)
			w.Write(mapB)
		} else {
			w.WriteHeader(retCode)
		}
	}).Methods(method)
	go func() {
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			panic(err)
		}
	}()
	<-stop
	err := srv.Close()
	if err != nil {
		panic(err)
	}
}
