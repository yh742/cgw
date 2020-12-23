package cgw

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
)

type ctxKey int
type requestType int

// Type of requests that we will see from client
const (
	EntityTokenReq   requestType = 0
	DisconnectionReq requestType = 1
)

// Type of values stored as ctx
const (
	DecodedJSON ctxKey = 0
)

// jsonDecoderHandler help decode handler and stuffs it into the context
func jsonDecodeHandler(reqType requestType, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// deocde json based on the request type specified
		var decodedReq interface{}
		switch reqType {
		case EntityTokenReq:
			decodedReq = &EntityTokenRequest{}
		case DisconnectionReq:
			decodedReq = &DisconnectRequest{}
		default:
			ErrorLog("request type is not specified")
			http.Error(w, "Interal Server Error", http.StatusInternalServerError)
			return
		}
		err := JSONDecodeRequest(w, req, 1<<12, decodedReq)
		if err != nil {
			ErrorLog("error occured decoding json, %s", err)
			return
		}
		chk, ok := decodedReq.(ValidityChecker)
		if !ok {
			ErrorLog("unable to cast decoded request body to validitychecker")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		if !chk.IsValid() {
			ErrorLog("request body missing a required field, %v", chk)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// put decoded JSON as part of context
		newCtx := context.WithValue(req.Context(), DecodedJSON, decodedReq)
		next(w, req.WithContext(newCtx))
	}
}

// redisLockHandler locks the key for a specific entity pair
// so concurrent requests on the same entity pair won't cause race condition
func redisLockHandler(rs RedisStore, timeout time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// retrieve json from body
		ctx := req.Context()
		dReq := ctx.Value(DecodedJSON)
		if dReq == nil {
			ErrorLog("unable to retrieve decoded json from ctx")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		eid, ok := dReq.(EntityIdentifier)
		if !ok {
			ErrorLog("unable to cast decoded json from ctx %+v", dReq)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// setup retries for locks
		backoff := redislock.LimitRetry(redislock.LinearBackoff(100*time.Millisecond), 3)
		lock, err := rs.redisLock.Obtain(ctx, "lock:"+eid.GetEntityPair().CreateKey(), timeout, &redislock.Options{
			RetryStrategy: backoff,
		})
		if err != nil {
			ErrorLog("unable to obtain lock for resource, %s, %s", eid.GetEntityPair().CreateKey(), err)
			http.Error(w, "Resource conflict", http.StatusConflict)
			return
		}
		defer lock.Release(ctx)
		next(w, req)
	}
}

// getReqFromContext retrieves value from context based on request type specified
func getReqFromContext(ctx context.Context, w http.ResponseWriter, reqType requestType, dataPtr interface{}) bool {
	value := ctx.Value(DecodedJSON)
	if value == nil {
		ErrorLog("unable to retrieve decoded json from ctx")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return false
	}
	switch reqType {
	case EntityTokenReq:
		lValPtr, ok := dataPtr.(*EntityTokenRequest)
		rVal, ok2 := value.(*EntityTokenRequest)
		if !ok || !ok2 {
			ErrorLog("unable to retrieve cast data from ctx, %t, %t", ok, ok2)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return false
		}
		*lValPtr = *rVal
	case DisconnectionReq:
		lValPtr, ok := dataPtr.(*DisconnectRequest)
		rVal, ok2 := value.(*DisconnectRequest)
		if !ok || !ok2 {
			ErrorLog("unable to retrieve cast data from ctx, %t, %t", ok, ok2)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return false
		}
		*lValPtr = *rVal
	}
	return true
}

// refreshToken is used to handle refresh calls, rewrites entityid/token to redis
// returns 200 on success
// returns 4xx for other errors
func refreshTokenHandler(rs RedisStore) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// get context and set in redis
		ctx := req.Context()
		tokeReq := &EntityTokenRequest{}
		if !getReqFromContext(ctx, w, EntityTokenReq, tokeReq) {
			return
		}
		exists, err := rs.redisClient.Exists(ctx, tokeReq.CreateKey()).Result()
		if exists == 0 {
			ErrorLog("token doesn't exist, %s", err)
			http.Error(w, "Internal server error", http.StatusNotFound)
			return
		} else if err != nil {
			ErrorLog("error occured getting token, %s", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		err = rs.redisClient.Set(ctx, tokeReq.CreateKey(), tokeReq.Token, 0).Err()
		if err != nil {
			ErrorLog("error occured setting token, %s", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// validateToken validates the entity/token mapping
// returns 200 on success
// returns 400 if it doesn't exist
// returns 4xx for other errors
func validateTokenHandler(rs RedisStore) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// create context and check with redis, redis must have the most up to date lookup
		ctx := req.Context()
		tokeReq := &EntityTokenRequest{}
		if !getReqFromContext(ctx, w, EntityTokenReq, tokeReq) {
			return
		}
		val, err := rs.redisClient.Get(ctx, tokeReq.CreateKey()).Result()
		if err == redis.Nil || val != tokeReq.Token {
			ErrorLog("user has no access, %+v", tokeReq)
			http.Error(w, "User does not have access", http.StatusForbidden)
			return
		} else if err != nil {
			ErrorLog("error occur getting key from redis, %+v", tokeReq)
			http.Error(w, "Error occured retrieving credentials", http.StatusInternalServerError)
			return
		}
		DebugLog("retrieved value %s from %s", val, tokeReq.CreateKey())
		w.WriteHeader(http.StatusOK)
	}
}

// createNewToken creates a new entity/token mapping on gateway
// returns 200 on success
// returns 409 if there's conflict
// returns 4xx for other errors
func createNewTokenHandler(rs RedisStore,
	endpoint string, mecID string, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// the entity ID send to us is the new entity ID that crs created
		// it will never be populated in cache, need to always check with caas first
		// create new request to send to caas
		ctx := req.Context()
		tokeReq := &EntityTokenRequest{}
		if !getReqFromContext(ctx, w, EntityTokenReq, tokeReq) {
			return
		}
		valReq := ValidateTokenRequest{
			EntityTokenRequest: *tokeReq,
			MEC:                mecID,
		}
		jsBytes, err := json.Marshal(valReq)
		if err != nil {
			ErrorLog("encoding json error, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}
		resp, err := HTTPRequest(ctx, "POST", endpoint,
			map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer " + token,
			}, nil, bytes.NewBuffer(jsBytes))
		if err != nil {
			ErrorLog("error occured making request to caas, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}
		// check response
		if resp.status == http.StatusOK {
			// write to cache and write OK to client
			err := rs.redisClient.Set(ctx, tokeReq.CreateKey(), tokeReq.Token, 0).Err()
			if err != nil {
				ErrorLog("error writing new entry to cache, %s", err.Error())
				http.Error(w, "Internal server cache write error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else if resp.status == http.StatusConflict {
			// we should only get here if the tokens match
			// check if json is formed correctly and not empty
			eID := &EntityPair{}
			err := json.Unmarshal(resp.body, eID)
			if err != nil {
				ErrorLog("decoding json response from caas filed, %s", err.Error())
				http.Error(w, "Internal server decoding error", http.StatusInternalServerError)
				return
			}
			if !eID.IsValid() {
				ErrorLog("received empty response from caas, entity exists, %s", err.Error())
				http.Error(w, "Internal server decoding error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusConflict)
			w.Write(resp.body)
		} else {
			ErrorLog("error response from caas %d", resp.status)
			http.Error(w, "Error occured upstream", resp.status)
		}
	}
}

// disconnectHandler disconnects the
func disconnectHandler(disconnecter Disconnecter,
	rs RedisStore, deleteIDEndpoint string,
	upstreamReasonCodes map[ReasonCode]bool, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// create context and try to disconnect first
		ctx := req.Context()
		disReq := &DisconnectRequest{}
		if !getReqFromContext(ctx, w, DisconnectionReq, disReq) {
			return
		}

		// (1) get key from redis
		token, err := rs.redisClient.Get(ctx, disReq.CreateKey()).Result()
		if err == redis.Nil {
			ErrorLog("entity does not exist, %s", disReq.CreateKey())
			http.Error(w, "Entity/EntityID does not exist", http.StatusNotFound)
			return
		} else if err != nil {
			ErrorLog("error response getting key from redis, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}

		skipped := false
		// (2) if needed, delete
		if _, ok := upstreamReasonCodes[disReq.ReasonCode]; ok {
			tokReq := EntityTokenRequest{
				EntityPair: disReq.EntityPair,
				Token:      token,
			}
			jsBytes, err := json.Marshal(tokReq)
			resp, err := HTTPRequest(ctx, "POST", deleteIDEndpoint,
				map[string]string{
					"Content-Type":  "application/json",
					"Authorization": "Bearer " + token,
				}, nil, bytes.NewBuffer(jsBytes))
			if err != nil {
				ErrorLog("unable to make request to caas, %v", err)
				http.Error(w, "Unable to make request to caas", http.StatusInternalServerError)
				return
			}
			if resp.status == http.StatusNotFound {
				// if entityid/token mapping is not found, swallow the error for now since its gone already
				ErrorLog("unable to find entityid, got back %d from caas, %s",
					resp.status, resp.body)
				skipped = true
			} else if resp.status != http.StatusOK {
				ErrorLog("bad response from caas, got back %d from caas, %s",
					resp.status, resp.body)
				http.Error(w, "Unable to make request to caas", http.StatusInternalServerError)
				return
			}
		}

		// (3) disconnect the client
		// Disconnect call should always return success even if there's nothing to delete
		err = disconnecter.Disconnect(ctx, *disReq)
		if err != nil {
			ErrorLog("disconnect error, %s", err.Error())
			http.Error(w, "Internal error occured while disconnecting", http.StatusInternalServerError)
			return
		}
		err = rs.redisClient.Del(ctx, disReq.CreateKey()).Err()
		if err != nil {
			ErrorLog("error deleting key from redis, %s", err)
			http.Error(w, "Internal error occured with key store", http.StatusInternalServerError)
			return
		}

		if skipped {
			w.Header().Add("caas-verification", "skipped")
		}
		w.WriteHeader(http.StatusOK)
	}
}
