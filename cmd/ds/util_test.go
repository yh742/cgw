package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"gotest.tools/assert"
)

type StubServer struct {
	reqHistory      [][]byte
	reqQueryHistory []*url.URL
}

func (ss *StubServer) RunStubServer(port string, method string, path string, retVal map[string]string, retCode int, stop <-chan struct{}) {
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    "localhost:" + port,
		Handler: router,
	}
	// router.HandleFunc("/registration", func(w http.ResponseWriter, r *http.Request) {
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
		for {
			err := srv.ListenAndServe()
			if err != nil && err.Error() == "listen tcp 127.0.0.1:8080: bind: address already in use" {
				log.Debug().Msg("ASDFSDFDS")
				continue
			} else if err != http.ErrServerClosed {
				log.Debug().Msgf("ASDFSDFDS2, %s", err.Error())
				panic(err)
			}
			break
		}
	}()
	<-stop
	srv.Close()
	// wait := time.Second * 10
	// ctx, cancel := context.WithTimeout(context.Background(), wait)
	// defer cancel()
	// srv.Shutdown(ctx)
}

func TestURLJoin(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string][2]string{
			"http://localhost:8080/ds/v1/refresh":  {"http://localhost:8080", "ds/v1/refresh"},
			"http://localhost:8080/ds/v1/refresh/": {"http://localhost:8080", "ds/v1/refresh/"},
		}
		for k, v := range testTable {
			joined, err := URLJoin(v[0], v[1])
			assert.NilError(t, err, k)
			assert.Equal(t, k, joined)
		}
	})

	t.Run("fail_case", func(t *testing.T) {
		testTable := map[string][2]string{
			"localhost:8080/ds/v1/refresh/": {"localhost:8080", "ds/v1/refresh/"},
		}
		for _, v := range testTable {
			_, err := URLJoin(v[0], v[1])
			assert.Error(t, err, "url format is incorrect, must specify protocol/scheme")
		}
	})

}

func TestHTTPRequest(t *testing.T) {
	stop := make(chan struct{})
	ss := StubServer{}
	go ss.RunStubServer("8080", "POST", "/test", map[string]string{"TEST": "OK"}, http.StatusOK, stop)
	defer func() {
		stop <- struct{}{}
	}()

	jBytes, err := json.Marshal(map[string]string{"test": "test"})
	assert.NilError(t, err)
	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))

	data, err := HTTPRequest("POST", "http://localhost:8080/test", map[string]string{"query": "test"}, byteReadCloser, http.StatusOK)
	assert.NilError(t, err)

	// test output of server
	jsonMap := &map[string]string{}
	err = json.Unmarshal(data, jsonMap)
	assert.NilError(t, err)
	assert.Equal(t, (*jsonMap)["TEST"], "OK")

	// test request body store of server
	reqVal := &map[string]string{}
	err = json.Unmarshal(ss.reqHistory[0], reqVal)
	assert.NilError(t, err)
	assert.Equal(t, (*reqVal)["test"], map[string]string{"test": "test"}["test"])

	// test query string store of server
	assert.Equal(t, ss.reqQueryHistory[0].String(), "/test?query=test")
}
