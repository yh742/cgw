package ds

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// GetTokenHandler returns a token based on entityID
func GetTokenHandler(w http.ResponseWriter, req *http.Request) {
	entityID := req.URL.Query().Get("entityid")
	if strings.TrimSpace(entityID) == "" {
		w.WriteHeader(http.StatusBadRequest)
	} else {

		bytes, err := json.Marshal(map[string]string{"token": "123456.123456"})
		if err != nil {
			log.Error().Msg("error occured marshalling this call")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

// RefreshTokenHandler refreshes a token based on the entityID/token passed in
func RefreshTokenHandler(w http.ResponseWriter, req *http.Request) {
	var rr EntityTokenPair
	err := json.NewDecoder(req.Body).Decode(&rr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(rr.EntityID) == "" || strings.TrimSpace(rr.Token) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// RegistrationHandler registers a new entity
func RegistrationHandler(w http.ResponseWriter, req *http.Request) {
	var jsonMap map[string]interface{}
	err := json.NewDecoder(req.Body).Decode(&jsonMap)
	if err != nil {
		log.Debug().Msg("sadfsdf")
		log.Debug().Msg(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(jsonMap) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Debug().Msgf("%v", jsonMap)
	w.WriteHeader(http.StatusCreated)
	jsBytes, err := json.Marshal(map[string]string{
		"ID": "12",
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(jsBytes)
}

// DeleteEntityHandler delete the entity id from the service
func DeleteEntityHandler(w http.ResponseWriter, req *http.Request) {
	var der DeleteEntityRequest
	//err := json.NewDecoder(req.Body).Decode(&der)
	bytess, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%s", err.Error())
		return
	}
	err = json.Unmarshal(bytess, &der)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%s", err.Error())
		return
	}
	if strings.TrimSpace(der.EntityID) == "" || strings.TrimSpace(der.Token) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// TestHandler returns input and outpu
func TestHandler(w http.ResponseWriter, req *http.Request) {
	queryString := req.URL.Query().Get("query")
	if strings.TrimSpace(queryString) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(queryString))
}

// LogRequestHandler logs requests
func (sm *ServiceMocks) LogRequestHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		bbytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sm.requestsHistory = append(sm.requestsHistory, RequestEntry{
			query: req.URL.String(),
			body:  bbytes,
		})
		req.Body = ioutil.NopCloser(bytes.NewReader(bbytes))
		next(w, req)
	}
}

// StartServer starts the server
func (sm *ServiceMocks) StartServer(port string, stop <-chan struct{}) {
	// routing
	sm.port = port
	router := mux.NewRouter()
	router.HandleFunc("/crs/v1/token", sm.LogRequestHandler(GetTokenHandler)).Methods("GET")
	router.HandleFunc("/crs/v1/refresh", sm.LogRequestHandler(RefreshTokenHandler)).Methods("POST")
	router.HandleFunc("/crs/v1/registration", sm.LogRequestHandler(RegistrationHandler)).Methods("POST")
	router.HandleFunc("/caas/v1/entity/delete", sm.LogRequestHandler(DeleteEntityHandler)).Methods("POST")
	router.HandleFunc("/", sm.LogRequestHandler(TestHandler)).Methods("POST")
	go func() {
		err := http.ListenAndServe("0.0.0.0:"+port, router)
		if err != http.ErrServerClosed {
			log.Error().Msgf("unable to start mock services server. %s", err)
			panic(err)
		}
	}()
	log.Debug().Msg("started mock services...")
	<-stop
}

// GetTail starts the server
func (sm *ServiceMocks) GetTail(index int) RequestEntry {
	return sm.requestsHistory[len(sm.requestsHistory)-index]
}

// RequestEntry is a request transaction
type RequestEntry struct {
	query string
	body  []byte
}

// ServiceMocks represents all the services that ds can talk to
type ServiceMocks struct {
	requestsHistory []RequestEntry
	port            string
}
