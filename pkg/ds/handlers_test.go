package ds

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
	"gotest.tools/assert"
)

type dsMock struct {
	DisconnectRequest
}

func (m *dsMock) Disconnect(ctx context.Context, ds DisconnectRequest) error {
	m.DisconnectRequest = ds
	return nil
}

// global mocks accessible to everyone
var sm ServiceMocks
var gw CAASGateway
var redMock redismock.ClientMock

func TestMain(m *testing.M) {
	// setup servers for unit testing
	sm = ServiceMocks{}
	var rClient *redis.Client
	rClient, redMock = redismock.NewClientMock()
	gw = CAASGateway{
		mecID:                 "local.mec",
		caasCreateURL:         "http://localhost:9090/caas/v1/token/entity",
		caasValidateURL:       "http://localhost:9090/caas/v1/token/validate",
		caasDeleteEntityIDURL: "http://localhost:9090/caas/v1/token/entity/delete",
		upstreamReasonCodes:   map[ReasonCode]bool{Idle: true, NotAuthorized: true},
		disconnecter:          &dsMock{},
		kv: RedisStore{
			redisClient: rClient,
			redisLock:   redislock.New(rClient),
		},
	}
	stop := make(chan struct{})
	go sm.StartServer("9090", stop)
	exitVal := m.Run()
	stop <- struct{}{}
	os.Exit(exitVal)
}

func createTestRequest(t *testing.T, inStruct interface{}) *http.Request {
	jsBytes, err := json.Marshal(inStruct)
	assert.NilError(t, err)
	req, err := http.NewRequest("POST", "random.url", bytes.NewBuffer(jsBytes))
	assert.NilError(t, err)
	req.Header.Add("Content-Type", "application/json")
	return req
}

func TestJSONDecodeHandler(t *testing.T) {
	// setup stuff here
	var lastReq *http.Request
	entHandler := jsonDecodeHandler(EntityTokenReq, func(w http.ResponseWriter, req *http.Request) { lastReq = req })
	entityTokenReq := EntityTokenRequest{
		Token: "test.token",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	}
	disHandler := jsonDecodeHandler(DisconnectionReq, func(w http.ResponseWriter, req *http.Request) { lastReq = req })
	dsReq := DisconnectRequest{
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
		NextServer: "rkln.mec",
		ReasonCode: Reauthenticate,
	}

	// check success cases
	t.Run("succcess", func(t *testing.T) {
		for inStruct, handler := range map[interface{}]http.HandlerFunc{
			entityTokenReq: entHandler,
			dsReq:          disHandler,
		} {
			req := createTestRequest(t, inStruct)
			w := &httptest.ResponseRecorder{}
			handler(w, req)

			// read back results and check if they're equal
			val := lastReq.Context().Value(DecodedJSON)
			assert.Assert(t, val != nil)
			switch val.(type) {
			case *EntityTokenRequest:
				readBack := val.(*EntityTokenRequest)
				assert.Equal(t, *readBack, inStruct)
			case *DisconnectRequest:
				readBack := val.(*DisconnectRequest)
				assert.Equal(t, *readBack, inStruct)
			default:
				assert.Assert(t, false, "errorneous type ")
			}
		}
	})

	// check json
	t.Run("bad_json", func(t *testing.T) {
		entityTokenReq.Token = ""
		req := createTestRequest(t, entityTokenReq)
		w := &httptest.ResponseRecorder{}
		entHandler(w, req)

		// read back results and check if they're equal
		assert.Equal(t, w.Code, http.StatusBadRequest)
	})

	// check bad handler initialization
	t.Run("bad_handler", func(t *testing.T) {
		badHandler := jsonDecodeHandler(requestType(2), func(w http.ResponseWriter, req *http.Request) { lastReq = req })
		req := createTestRequest(t, map[string]string{"x": "test"})
		w := &httptest.ResponseRecorder{}
		badHandler(w, req)
		assert.Equal(t, w.Code, http.StatusInternalServerError)
	})
}

func TestRedisLock(t *testing.T) {
	x := 0
	lockHandler := redisLockHandler(gw.kv, 5*time.Second, func(w http.ResponseWriter, req *http.Request) {
		x++
		DebugLog("fufu")
		time.Sleep(2 * time.Second)
	})
	req := createTestRequest(t, struct{}{})
	ctx := context.WithValue(req.Context(), DecodedJSON, &EntityTokenRequest{
		Token: "test.token",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	})
	redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, 5*time.Second).SetVal(true)
	redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, 5*time.Second).SetVal(false)
	redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, 5*time.Second).SetVal(false)
	redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, 5*time.Second).SetVal(false)
	redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, 5*time.Second).SetVal(false)
	w := &httptest.ResponseRecorder{}
	req = req.WithContext(ctx)
	go lockHandler(w, req)
	lockHandler(w, req)
	assert.Equal(t, x, 1)
	assert.Equal(t, w.Code, http.StatusConflict)
}

func TestGetReqFromContext(t *testing.T) {
	for reqType, inStruct := range map[requestType]interface{}{
		EntityTokenReq: &EntityTokenRequest{
			Token: "test.token",
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
		},
		DisconnectionReq: &DisconnectRequest{
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
			NextServer: "rkln.mec",
			ReasonCode: Reauthenticate,
		},
	} {
		req := createTestRequest(t, struct{}{})
		ctx := context.WithValue(req.Context(), DecodedJSON, inStruct)
		w := &httptest.ResponseRecorder{}
		if reqType == EntityTokenReq {
			st := &EntityTokenRequest{}
			b := getReqFromContext(ctx, w, reqType, st)
			assert.Assert(t, b)
			assert.Equal(t, st.Token, "test.token")
		} else if reqType == DisconnectionReq {
			st := &DisconnectRequest{}
			b := getReqFromContext(ctx, w, reqType, st)
			assert.Assert(t, b)
			assert.Equal(t, st.NextServer, "rkln.mec")
		}

	}
}

// func TestRefreshToken(t *testing.T) {
// 	httpHandler := http.TimeoutHandler(refreshToken(gw.kv), time.Second, "Timed out")
// 	defer func() {
// 		gw.kv.Flush(context.Background())
// 		redMock.ClearExpect()
// 	}()
// 	// common structs
// 	valStruct := ValidateTokenRequest{
// 		EntityTokenRequest: EntityTokenRequest{
// 			Entity: "veh",
// 			Token:  "test.test",
// 			EntityIDStruct: EntityIDStruct{
// 				"1234",
// 			},
// 		},
// 		MEC: gw.mecID,
// 	}

// 	redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
// 	writer := httptest.NewRecorder()
// 	req := createTestRequest(t, valStruct.EntityTokenRequest)
// 	httpHandler.ServeHTTP(writer, req)
// 	assert.Equal(t, writer.Code, http.StatusOK)
// }

// func TestValidateToken(t *testing.T) {
// 	httpHandler := http.TimeoutHandler(validateToken(gw.kv, gw.caasValidateURL, gw.mecID), time.Second, "Timed out")
// 	defer func() {
// 		gw.kv.Flush(context.Background())
// 		redMock.ClearExpect()
// 	}()
// 	// common structs
// 	valStruct := ValidateTokenRequest{
// 		EntityTokenRequest: EntityTokenRequest{
// 			Entity: "veh",
// 			Token:  "test.test",
// 			EntityIDStruct: EntityIDStruct{
// 				"1235",
// 			},
// 		},
// 		MEC: gw.mecID,
// 	}

// 	redMock.ExpectSet("veh-1235", "test.test", 0).SetVal("")
// 	// pre-populated a entry in redis cache
// 	gw.kv.Set(context.Background(), "veh-1235", "test.test")

// 	t.Run("verify_through_cached", func(t *testing.T) {
// 		redMock.ExpectGet("veh-1235").SetVal("test.test")
// 		writer := httptest.NewRecorder()
// 		req := createTestRequest(t, valStruct.EntityTokenRequest)
// 		httpHandler.ServeHTTP(writer, req)
// 		assert.Equal(t, writer.Code, http.StatusOK)
// 	})

// 	// t.Run("verify_through_caas", func(t *testing.T) {
// 	// 	writer := httptest.NewRecorder()
// 	// 	redMock.ExpectGet("veh-4321").RedisNil()
// 	// 	redMock.ExpectSet("veh-4321", "test.test", 0).SetVal("")
// 	// 	valStruct.EntityTokenRequest.EntityID = "4321"
// 	// 	req := createTestRequest(t, valStruct.EntityTokenRequest)
// 	// 	httpHandler.ServeHTTP(writer, req)
// 	// 	assert.Equal(t, writer.Code, http.StatusOK)
// 	// })

// 	t.Run("fail_case", func(t *testing.T) {
// 		writer := httptest.NewRecorder()
// 		redMock.ExpectGet("veh-5321").RedisNil()
// 		// redMock.ExpectSet("veh-4321", "test.test", 0).SetVal("")
// 		valStruct.EntityTokenRequest.Token = "fail.test"
// 		req := createTestRequest(t, valStruct.EntityTokenRequest)
// 		httpHandler.ServeHTTP(writer, req)
// 		assert.Equal(t, writer.Code, http.StatusForbidden)
// 	})
// }

// func TestCreateNewToken(t *testing.T) {
// 	// setup http handler
// 	httpHandler := http.TimeoutHandler(createNewToken(gw.kv, gw.caasCreateURL, gw.mecID), time.Second, "Timed out")
// 	defer func() {
// 		gw.kv.Flush(context.Background())
// 		redMock.ClearExpect()
// 	}()
// 	// common structs
// 	valStruct := ValidateTokenRequest{
// 		EntityTokenRequest: EntityTokenRequest{
// 			Entity: "veh",
// 			Token:  "test.test",
// 			EntityIDStruct: EntityIDStruct{
// 				"1234",
// 			},
// 		},
// 		MEC: gw.mecID,
// 	}

// 	// test cases:
// 	// (1) run success case
// 	// (2) run conflict case
// 	// (3) run http timeout case
// 	// (4) run fail case
// 	t.Run("success_case", func(t *testing.T) {
// 		// set expectations
// 		redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")

// 		// create request and run
// 		writer := httptest.NewRecorder()
// 		req := createTestRequest(t, valStruct.EntityTokenRequest)
// 		httpHandler.ServeHTTP(writer, req)

// 		// check:
// 		// (1) if caas got the correct request
// 		// (2) if caas responded with OK and set KV
// 		// (3) if returned OK status
// 		jsBytes, err := json.Marshal(valStruct)
// 		assert.NilError(t, err)
// 		assert.Equal(t, writer.Result().StatusCode, http.StatusOK)
// 		assert.Equal(t, string(sm.GetTail(1).body), string(jsBytes))
// 	})

// 	t.Run("conflict_case", func(t *testing.T) {
// 		// set expectations
// 		redMock.ExpectSet("veh-4321", "repeated.test", 0).SetVal("")

// 		// create request and run
// 		writer := httptest.NewRecorder()
// 		valStruct.EntityTokenRequest.Token = "repeated.test"
// 		req := createTestRequest(t, valStruct.EntityTokenRequest)
// 		httpHandler.ServeHTTP(writer, req)

// 		// check:
// 		// (1) 409 conflict is received
// 		// (2) bytes unmarshalled to json properly
// 		jsBytes, err := json.Marshal(valStruct)
// 		assert.NilError(t, err)
// 		assert.Equal(t, string(sm.GetTail(1).body), string(jsBytes))
// 		assert.Equal(t, writer.Result().StatusCode, http.StatusConflict)
// 	})

// 	t.Run("fail_case_timeout", func(t *testing.T) {
// 		// create request and run
// 		writer := httptest.NewRecorder()
// 		valStruct.EntityTokenRequest.Token = "sleep.test"
// 		req := createTestRequest(t, valStruct.EntityTokenRequest)
// 		httpHandler.ServeHTTP(writer, req)

// 		// check if timed out error is throw (from timeout handler)
// 		assert.Equal(t, writer.Result().StatusCode, http.StatusServiceUnavailable)
// 		assert.Equal(t, string(writer.Body.Bytes()), "Timed out")
// 	})

// 	t.Run("fail_case", func(t *testing.T) {
// 		// create fail request and run
// 		writer := httptest.NewRecorder()
// 		valStruct.EntityTokenRequest.Token = "fail.test"
// 		req := createTestRequest(t, valStruct.EntityTokenRequest)
// 		httpHandler.ServeHTTP(writer, req)

// 		// check:
// 		// (1) 409 conflict is received
// 		// (2) bytes unmarshalled to json properly
// 		jsBytes, err := json.Marshal(valStruct)
// 		assert.NilError(t, err)
// 		assert.Equal(t, string(sm.GetTail(1).body), string(jsBytes))
// 		assert.Equal(t, writer.Result().StatusCode, http.StatusBadRequest)
// 	})
// }

// func TestDisconnectHandler(t *testing.T) {
// 	ds := &dsMock{}
// 	jBytes, err := json.Marshal(DisconnectRequest{
// 		Entity:     "sw",
// 		EntityID:   "12",
// 		ReasonCode: 0x9C,
// 		NextServer: "localhost:8080",
// 	})
// 	assert.NilError(t, err)
// 	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))
// 	handler := DisconnectHandler(ds)
// 	handler(nil, &http.Request{
// 		Body: byteReadCloser,
// 	})
// 	assert.Equal(t, ds.Entity, "sw")
// 	assert.Equal(t, ds.EntityID, "12")
// 	assert.Equal(t, ds.NextServer, "localhost:8080")
// 	assert.Equal(t, ds.ReasonCode, ReasonCode(0x9C))
// }
