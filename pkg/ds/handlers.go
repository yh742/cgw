package ds

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// refreshToken is used to handle refresh calls, rewrites entityid/token to redis
func refreshToken(kv KeyValueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// deocde json from body
		tokenReq := EntityTokenRequest{}
		err := JSONDecodeRequest(w, req, 1<<12, &tokenReq)
		if err != nil {
			log.Error().Msgf("refreshToken: error occured decoding json, %s", err)
			return
		}
		// create context and set in redis
		ctx := req.Context()
		err = kv.Set(ctx, HyphenConcat(tokenReq.Entity, tokenReq.EntityID), tokenReq.Token)
		if err != nil {
			log.Error().Msgf("refreshToken: error occured setting token, %s", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func validateToken(kv KeyValueStore, endpoint string, mecID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// decode json from body
		tokenReq := EntityTokenRequest{}
		err := JSONDecodeRequest(w, req, 1<<12, &tokenReq)
		if err != nil {
			// only log here, JSONDecodeRequests writes to responsewriter
			log.Error().Msgf("validateToken: error occured decoding json, %s", err)
			return
		}

		// create context and check with redis, redis must have the most up to date lookup
		ctx := req.Context()
		val, err := kv.Get(ctx, HyphenConcat(tokenReq.Entity, tokenReq.EntityID))
		if err == redis.Nil || val != tokenReq.Token {
			log.Error().Msgf("validateToken: no access, %+v", tokenReq)
			http.Error(w, "User does not have access", http.StatusForbidden)
			return
		} else if err != nil {
			log.Error().Msgf("validateToken: error occur getting from redis, %+v", tokenReq)
			http.Error(w, "Error occured retrieving credentials", http.StatusInternalServerError)
			return
		}
		log.Debug().Msgf("validateToken: retrieved value %s from %s", val, HyphenConcat(tokenReq.Entity, tokenReq.EntityID))
		w.WriteHeader(http.StatusOK)

		/* 	THIS CASE SHOULDN'T APPLY AS REDIS SHOULD ALWAYS BE IN SYNC
		// cache might not be up to date, check with server
		valReq := ValidateTokenRequest{
			EntityTokenRequest: tokenReq,
			MEC:                mecID,
		}
		jsBytes, err := json.Marshal(valReq)
		if err != nil {
			log.Error().Msgf("validateToken: encoding json error, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}
		resp, err := HTTPRequest(ctx, "POST", endpoint, map[string]string{"Content-Type": "application/json"}, nil, bytes.NewBuffer(jsBytes))
		if err != nil {
			log.Error().Msgf("validateToken: error occured making request to caas, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}
		if resp.status == http.StatusOK {
			err := kv.Set(ctx, HyphenConcat(tokenReq.Entity, tokenReq.EntityID), tokenReq.Token)
			if err != nil {
				log.Error().Msgf("validateToken: error in setting redis value, %s", err)
				http.Error(w, "Error occured with service", http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusOK)
		} else {
			log.Error().Msgf("validateToken: error returned from caas %d, %s", resp.status, string(resp.body))
			w.WriteHeader(resp.status)
			w.Write(resp.body)
		}
		*/
	})
}

func createNewToken(kv KeyValueStore, endpoint string, mecID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// decode json from body
		tokenReq := EntityTokenRequest{}
		err := JSONDecodeRequest(w, req, 1<<12, &tokenReq)
		if err != nil {
			log.Error().Msgf("createNewToken: error occured decoding json, %s", err)
			return
		}

		// the entity ID send to us is the new entity ID that crs created
		// it will never be populated in cache, need to always check with caas first
		// create new request to send to caas
		ctx := req.Context()
		valReq := ValidateTokenRequest{
			EntityTokenRequest: tokenReq,
			MEC:                mecID,
		}
		jsBytes, err := json.Marshal(valReq)
		if err != nil {
			log.Error().Msgf("createNewToken: encoding json error, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}
		resp, err := HTTPRequest(ctx, "POST", endpoint, map[string]string{"Content-Type": "application/json"}, nil, bytes.NewBuffer(jsBytes))
		if err != nil {
			log.Error().Msgf("createNewToken: error occured making request to caas, %s", err)
			http.Error(w, "Error occured upstream", http.StatusInternalServerError)
			return
		}
		// check response
		if resp.status == http.StatusOK {
			// write to cache and write OK to client
			err := kv.Set(ctx, HyphenConcat(tokenReq.Entity, tokenReq.EntityID), tokenReq.Token)
			if err != nil {
				log.Error().Msgf("createNewToken: error writing new entry to cache, %s", err.Error())
				http.Error(w, "Internal server cache write error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else if resp.status == http.StatusConflict {
			// check if json is formed correctly and not empty
			eID := EntityIDStruct{}
			err := json.Unmarshal(resp.body, &eID)
			if err != nil {
				log.Error().Msgf("createNewToken: decoding json response from caas filed, %s", err.Error())
				http.Error(w, "Internal server decoding error", http.StatusInternalServerError)
				return
			}
			if IsEmpty(eID.EntityID) {
				log.Error().Msgf("createNewToken: response from caas is empty")
				http.Error(w, "Internal server decoding error", http.StatusInternalServerError)
				return
			}
			// write to cache and write 409 to client w/ the existin entity id
			err = kv.Set(ctx, HyphenConcat(tokenReq.Entity, eID.EntityID), tokenReq.Token)
			if err != nil {
				log.Error().Msgf("createNewToken: error writing new entry to cache, %s", err.Error())
				http.Error(w, "Internal server cache update error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusConflict)
			w.Write(resp.body)
		} else {
			log.Error().Msgf("createNewToken: error response from caas %d", resp.status)
			http.Error(w, "Error occured upstream", resp.status)
		}
	})
}

// deleteEntityID looks up the token based on entity ID and delete its from CAAS
func deleteEntityID(entityID string, reasonCode ReasonCode) error {
	// check if reasoncode requires a upstream delete call to CAAS
	// if _, ok := upstreamReasonCodes[reasonCode]; !ok {
	// 	return nil
	// }
	// data, err := HTTPRequest("GET", getTokenEndpoint, nil, map[string]string{"entityid": entityID}, nil, http.StatusOK)
	// if err != nil {
	// 	log.Error().Msgf("unable to make request, %v", err)
	// 	return errors.New("unable to make request")
	// }
	// tokenStruct := &struct {
	// 	Token string `json:"token,required"`
	// }{}
	// err = json.Unmarshal(data, tokenStruct)
	// if err != nil {
	// 	log.Error().Msgf("unable to unmarshal data, '%s', %v", data, err)
	// 	return errors.New("unable to marshal data")
	// }
	// deleteReq := DeleteEntityRequest{
	// 	Entity: "veh",
	// 	EntityTokenPair: EntityTokenPair{
	// 		EntityID: entityID,
	// 		Token:    tokenStruct.Token,
	// 	},
	// }
	// jBytes, err := json.Marshal(deleteReq)
	// if err != nil {
	// 	log.Error().Msgf("unable to marshal delete entity request, %v", err)
	// 	return errors.New("unable to marshal delete entity request")
	// }

	// data, err = HTTPRequest("POST", deleteEntityIDEndpoint, nil, nil, bytes.NewBuffer(jBytes), http.StatusOK)
	// if err != nil {
	// 	log.Error().Msgf("unable to make delete entity id request, %v", err)
	// 	return errors.New("unable to make delete entity id request")
	// }
	return nil
}

// DisconnectHandler wraps all the disconnect calls to extract request parameter
func DisconnectHandler(disconnector Disconnecter) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		connReq := &DisconnectRequest{}
		err := json.NewDecoder(req.Body).Decode(connReq)
		if err != nil {
			log.Error().Msgf("error decoding json request %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Debug().Msgf("received client request, %+v", connReq)
		// perform lookup of token based on entityID
		err = deleteEntityID(connReq.EntityID, connReq.ReasonCode)
		if err != nil {
			log.Error().Msgf("error looking up token based on json request %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		disconnector.Disconnect(*connReq, w)
	}
}
