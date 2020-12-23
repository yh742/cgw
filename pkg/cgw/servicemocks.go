package cgw

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
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
	DebugLog("%v", jsonMap)
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
func CreateTokenHashHandler(sm *ServiceMocks) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var vr ValidateTokenRequest
		err := json.NewDecoder(req.Body).Decode(&vr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			ErrorLog("%s", err.Error())
			return
		}
		if IsEmpty(vr.Entity) || IsEmpty(vr.EntityID) || IsEmpty(vr.MEC) || IsEmpty(vr.Token) {
			w.WriteHeader(http.StatusBadRequest)
			ErrorLog("%+v", vr)
			return
		}
		// return different failure responses based on token
		if vr.Token == "sleep.test" {
			time.Sleep(10 * time.Second)
			return
		} else if vr.Token == "fail.test" {
			w.WriteHeader(http.StatusBadRequest)
		}
		DebugLog("%+v", vr)
		sm.lock.Lock()
		defer sm.lock.Unlock()
		if val, ok := sm.db[vr.Token]; ok {
			// token creation request is repeated
			DebugLog("key exists already, %s", val)
			jbytes, err := json.Marshal(val)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				ErrorLog("%s", err.Error())
				return
			}
			DebugLog("returning conflict status")
			w.WriteHeader(http.StatusConflict)
			w.Write(jbytes)
		} else {
			// create new token
			sm.db[vr.Token] = vr.EntityPair
			w.WriteHeader(http.StatusOK)
		}
	}
}

// DeleteEntityHandler delete the entity id from the service
func DeleteEntityHandler(sm *ServiceMocks) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var der EntityTokenRequest
		err := json.NewDecoder(req.Body).Decode(&der)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			ErrorLog("%s", err.Error())
			return
		}
		if !der.IsValid() {
			w.WriteHeader(http.StatusBadRequest)
			ErrorLog("%+v", der)
			return
		}
		sm.lock.Lock()
		defer sm.lock.Unlock()
		if _, ok := sm.db[der.Token]; !ok {
			w.WriteHeader(http.StatusNotFound)
			ErrorLog("%+v", der)
			return
		}
		delete(sm.db, der.Token)
		DebugLog("token deleted, %+v", der)
		DebugLog("db status, %+v", sm.db)
		w.WriteHeader(http.StatusOK)
	}
}

// DeleteHandler removes entries
func DeleteHandler(sm *ServiceMocks) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sm.lock.Lock()
		defer sm.lock.Unlock()
		sm.ClearDB()
		DebugLog("deleted db, %v", sm.db)
		w.WriteHeader(http.StatusAccepted)
	}
}

// TestHandler returns input and output
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
	db              map[string]EntityPair
	requestsHistory []RequestEntry
	port            string
	lock            sync.Mutex
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
	sm.db = map[string]EntityPair{}
	router := mux.NewRouter()
	// crs endpoints
	router.HandleFunc("/crs/v1/registration", sm.LogRequestHandler(RegistrationHandler)).Methods("POST")

	// caas endpoints
	router.HandleFunc("/caas/v1/token/entity", sm.LogRequestHandler(CreateTokenHashHandler(sm))).Methods("POST")
	router.HandleFunc("/caas/v1/token/entity/delete", sm.LogRequestHandler(DeleteEntityHandler(sm))).Methods("POST")
	router.HandleFunc("/", sm.LogRequestHandler(DeleteHandler(sm))).Methods("DELETE")
	router.HandleFunc("/", sm.LogRequestHandler(TestHandler)).Methods("POST")
	go func() {
		err := http.ListenAndServe("0.0.0.0:"+port, router)
		if err != http.ErrServerClosed {
			ErrorLog("unable to start mock services server. %s", err)
			panic(err)
		}
	}()
	DebugLog("started mock services...")
	<-stop
}

// GetTail starts the server
func (sm *ServiceMocks) GetTail(index int) RequestEntry {
	return sm.requestsHistory[len(sm.requestsHistory)-index]
}

// ClearDB removes all entries in db
func (sm *ServiceMocks) ClearDB() {
	sm.db = map[string]EntityPair{}
	DebugLog("removed db, %v", sm.db)
}
