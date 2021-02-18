package cgw

import (
	"encoding/json"
	"net/http"
)

type readTokenCb func() string
type writeTokenCb func(string)
type readMECCb func() string
type writeMECCb func(string)
type appendLogCb func(string, interface{})
type getLogsCb func() []interface{}
type clearLogsCb func()

func flushHandler(kv RedisStore) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := kv.redisClient.FlushAll(req.Context()).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func setTokenHandler(setToken writeTokenCb) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		token := req.URL.Query().Get("token")
		setToken(token)
		DebugLog("set token to value: %s", token)
		w.WriteHeader(http.StatusOK)
	}
}

func setMECHandler(setMEC writeMECCb) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		mec := req.URL.Query().Get("mec")
		setMEC(mec)
		DebugLog("set mec to value: %s", mec)
		w.WriteHeader(http.StatusOK)
	}
}

func getReqLogHandler(getLogs getLogsCb) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		DebugLog("getting request log")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(getLogs())
	}
}

func delReqLogHandler(clear clearLogsCb) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		clear()
		w.WriteHeader(http.StatusNoContent)
	}
}
