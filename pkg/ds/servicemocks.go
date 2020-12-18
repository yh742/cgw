package ds

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// RegistrationHandler registers a new entity
func RegistrationHandler(w http.ResponseWriter, req *http.Request) {
	var jsonMap map[string]interface{}
	err := json.NewDecoder(req.Body).Decode(&jsonMap)
	if err != nil {
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

// CreateTokenHashHandler validates token/entity/entityID
// uses the token to determin the type of response we will get
// repeated.test => returns 409 and entityID of existing entityID
// sleep.test => will sleep for 10 seconds
// fail.test => return an error status
func CreateTokenHashHandler(w http.ResponseWriter, req *http.Request) {
	var vr ValidateTokenRequest
	err := json.NewDecoder(req.Body).Decode(&vr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%s", err.Error())
		return
	}
	if IsEmpty(vr.Entity) || IsEmpty(vr.EntityID) || IsEmpty(vr.MEC) || IsEmpty(vr.Token) {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%+v", vr)
		return
	}
	// return different responses based on token
	if vr.Token == "repeated.test" {
		jbytes, err := json.Marshal(EntityPair{
			Entity:   "veh",
			EntityID: "4321",
		})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Error().Msgf("%s", err.Error())
			return
		}
		w.WriteHeader(http.StatusConflict)
		w.Write(jbytes)
	} else if vr.Token == "sleep.test" {
		time.Sleep(10 * time.Second)
		return
	} else if vr.Token == "fail.test" {
		w.WriteHeader(http.StatusBadRequest)
	}
	log.Debug().Msgf("%+v", vr)
	w.WriteHeader(http.StatusOK)
}

// ValidateHandler validates the token/entity/entityID
// fail.test => return an error status
func ValidateHandler(w http.ResponseWriter, req *http.Request) {
	var vr ValidateTokenRequest
	err := json.NewDecoder(req.Body).Decode(&vr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%s", err.Error())
		return
	}
	if IsEmpty(vr.Entity) || IsEmpty(vr.EntityID) || IsEmpty(vr.MEC) || IsEmpty(vr.Token) {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%+v", vr)
		return
	}
	if vr.Token == "fail.test" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}
	log.Debug().Msgf("%+v", vr)
	w.WriteHeader(http.StatusOK)
}

// DeleteEntityHandler delete the entity id from the service
func DeleteEntityHandler(w http.ResponseWriter, req *http.Request) {
	var der EntityTokenRequest
	err := json.NewDecoder(req.Body).Decode(&der)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%s", err.Error())
		return
	}
	if IsEmpty(der.EntityID) || IsEmpty(der.Token) {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Msgf("%+v", der)
		return
	}
	log.Debug().Msgf("%+v", der)
	w.WriteHeader(http.StatusOK)
}

// TestHandler returns input and outpu
func TestHandler(w http.ResponseWriter, req *http.Request) {
	queryString := req.URL.Query().Get("query")
	if IsEmpty(queryString) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(queryString))
}

// RequestEntry is a request transaction
type RequestEntry struct {
	header http.Header
	query  string
	body   []byte
}

// ServiceMocks represents all the services that ds can talk to
type ServiceMocks struct {
	requestsHistory []RequestEntry
	port            string
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
			header: req.Header,
			query:  req.URL.String(),
			body:   bbytes,
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
	// crs endpoints
	router.HandleFunc("/crs/v1/registration", sm.LogRequestHandler(RegistrationHandler)).Methods("POST")

	// caas endpoints
	router.HandleFunc("/caas/v1/token/entity", sm.LogRequestHandler(CreateTokenHashHandler)).Methods("POST")
	router.HandleFunc("/caas/v1/token/validate", sm.LogRequestHandler(ValidateHandler)).Methods("POST")
	router.HandleFunc("/caas/v1/token/entity/delete", sm.LogRequestHandler(DeleteEntityHandler)).Methods("POST")
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
