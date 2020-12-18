package ds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	if ds.ReasonCode == RateTooHigh {
		return errors.New("mqtt error")
	}
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

func createTestRequest(t *testing.T, bodyStruct interface{}, ctxStruct interface{}) *http.Request {
	var req *http.Request
	var err error
	if bodyStruct != nil {
		jsBytes, err := json.Marshal(bodyStruct)
		assert.NilError(t, err)
		req, err = http.NewRequest("POST", "random.url", bytes.NewBuffer(jsBytes))
		assert.NilError(t, err)
	} else {
		req, err = http.NewRequest("POST", "random.url", nil)
		assert.NilError(t, err)
	}
	req.Header.Add("Content-Type", "application/json")
	if ctxStruct != nil {
		ctx := context.WithValue(req.Context(), DecodedJSON, ctxStruct)
		req = req.WithContext(ctx)
	}
	return req
}

func TestTimeoutHandler(t *testing.T) {
	handler := http.TimeoutHandler(
		createNewTokenHandler(gw.kv, gw.caasValidateURL, gw.mecID),
		1*time.Millisecond,
		"Timed out")
	// create request and run
	etr := &EntityTokenRequest{
		Token: "sleep.test",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	}
	w := httptest.NewRecorder()
	req := createTestRequest(t, nil, etr)
	handler.ServeHTTP(w, req)

	// check if timed out error is throw (from timeout handler)
	assert.Equal(t, w.Result().StatusCode, http.StatusServiceUnavailable)
	assert.Equal(t, string(w.Body.Bytes()), "Timed out")
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
			req := createTestRequest(t, inStruct, nil)
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
		req := createTestRequest(t, entityTokenReq, nil)
		w := &httptest.ResponseRecorder{}
		entHandler(w, req)

		// read back results and check if they're equal
		assert.Equal(t, w.Code, http.StatusBadRequest)
	})

	// check bad handler initialization
	t.Run("bad_handler", func(t *testing.T) {
		badHandler := jsonDecodeHandler(requestType(2), func(w http.ResponseWriter, req *http.Request) { lastReq = req })
		req := createTestRequest(t, map[string]string{"x": "test"}, nil)
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
	req := createTestRequest(t, nil, &EntityTokenRequest{
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
	defer redMock.ClearExpect()
	w := &httptest.ResponseRecorder{}
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
		req := createTestRequest(t, nil, inStruct)
		w := &httptest.ResponseRecorder{}
		if reqType == EntityTokenReq {
			st := &EntityTokenRequest{}
			b := getReqFromContext(req.Context(), w, reqType, st)
			assert.Assert(t, b)
			assert.Equal(t, st.Token, "test.token")
		} else if reqType == DisconnectionReq {
			st := &DisconnectRequest{}
			b := getReqFromContext(req.Context(), w, reqType, st)
			assert.Assert(t, b)
			assert.Equal(t, st.NextServer, "rkln.mec")
		}

	}
}

func TestRefreshToken(t *testing.T) {
	handler := refreshTokenHandler(gw.kv)
	etr := &EntityTokenRequest{
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
		Token: "test.test",
	}
	redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
	defer redMock.ClearExpect()
	writer := httptest.NewRecorder()
	req := createTestRequest(t, nil, etr)
	handler(writer, req)
	assert.Equal(t, writer.Code, http.StatusOK)
}

func TestValidateToken(t *testing.T) {
	handler := validateTokenHandler(gw.kv, gw.caasValidateURL, gw.mecID)
	etr := &EntityTokenRequest{
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
		Token: "test.test",
	}

	// preset redis with a value to validate against
	redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
	defer redMock.ClearExpect()
	err := gw.kv.redisClient.Set(context.Background(), "veh-1234", "test.test", 0).Err()
	assert.NilError(t, err)

	t.Run("success_verify", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		defer redMock.ClearExpect()
		writer := httptest.NewRecorder()
		req := createTestRequest(t, nil, etr)
		handler(writer, req)
		assert.Equal(t, writer.Code, http.StatusOK)
	})

	t.Run("fail_verify", func(t *testing.T) {
		writer := httptest.NewRecorder()
		redMock.ExpectGet("sw-1234").RedisNil()
		defer redMock.ClearExpect()
		etr.Entity = "sw"
		req := createTestRequest(t, nil, etr)
		handler(writer, req)
		assert.Equal(t, writer.Code, http.StatusForbidden)
	})
}

func TestCreateNewToken(t *testing.T) {
	// setup http handler
	handler := createNewTokenHandler(gw.kv, gw.caasCreateURL, gw.mecID)
	etr := &EntityTokenRequest{
		Token: "test.test",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	}

	t.Run("success_case", func(t *testing.T) {
		// set expectations
		redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
		defer redMock.ClearExpect()

		// create request and run
		writer := httptest.NewRecorder()
		req := createTestRequest(t, nil, etr)
		handler(writer, req)

		// check results
		val := &ValidateTokenRequest{}
		err := json.Unmarshal(sm.GetTail(1).body, val)
		assert.NilError(t, err)
		assert.Equal(t, writer.Result().StatusCode, http.StatusOK)
		assert.Equal(t, val.MEC, "local.mec")
	})

	t.Run("conflict_case", func(t *testing.T) {
		// create request and run
		writer := httptest.NewRecorder()
		etr.Token = "repeated.test"
		req := createTestRequest(t, nil, etr)
		handler(writer, req)

		// check results
		val := &ValidateTokenRequest{}
		err := json.Unmarshal(sm.GetTail(1).body, val)
		assert.NilError(t, err)
		assert.Equal(t, writer.Result().StatusCode, http.StatusConflict)
		assert.Equal(t, val.CreateKey(), "veh-1234")
	})

	t.Run("fail_case", func(t *testing.T) {
		// create fail request and run
		writer := httptest.NewRecorder()
		etr.Token = "fail.test"
		req := createTestRequest(t, nil, etr)
		handler(writer, req)

		// check results
		assert.Equal(t, writer.Result().StatusCode, http.StatusBadRequest)
	})
}

func TestDisconnectHandler(t *testing.T) {
	ds := &dsMock{}
	handler := disconnectHandler(ds, gw.kv, gw.caasDeleteEntityIDURL, map[ReasonCode]bool{
		Idle:          true,
		NotAuthorized: true,
	})
	dr := &DisconnectRequest{
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
		ReasonCode: Reauthenticate,
		NextServer: "localhost:8080",
	}

	t.Run("success", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		redMock.ExpectDel("veh-1234").SetVal(1)
		defer redMock.ClearExpect()
		w := httptest.NewRecorder()
		req := createTestRequest(t, nil, dr)
		handler(w, req)
		defer assert.Equal(t, w.Code, http.StatusOK)
	})

	t.Run("success_with_upstream", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		redMock.ExpectDel("veh-1234").SetVal(1)
		defer redMock.ClearExpect()
		dr.ReasonCode = Idle
		w := httptest.NewRecorder()
		req := createTestRequest(t, nil, dr)
		handler(w, req)
		assert.Equal(t, w.Code, http.StatusOK)
	})

	t.Run("success_with_upstream_caas_fail", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("not.found.test")
		redMock.ExpectDel("veh-1234").SetVal(1)
		defer redMock.ClearExpect()
		dr.ReasonCode = Idle
		w := httptest.NewRecorder()
		req := createTestRequest(t, nil, dr)
		handler(w, req)
		assert.Equal(t, w.Code, http.StatusOK)
		assert.Equal(t, w.Header().Get("caas-verification"), "skipped")
	})

	t.Run("fail_missing_key", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetErr(redis.Nil)
		defer redMock.ClearExpect()
		w := httptest.NewRecorder()
		req := createTestRequest(t, nil, dr)
		handler(w, req)
		assert.Equal(t, w.Code, http.StatusNotFound)
	})

	t.Run("fail_caas", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("fail.test")
		defer redMock.ClearExpect()
		dr.ReasonCode = Idle
		w := httptest.NewRecorder()
		req := createTestRequest(t, nil, dr)
		handler(w, req)
		assert.Equal(t, w.Code, http.StatusInternalServerError)
	})

	t.Run("fail_disconnect", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		defer redMock.ClearExpect()
		dr.ReasonCode = RateTooHigh
		w := httptest.NewRecorder()
		req := createTestRequest(t, nil, dr)
		handler(w, req)
		assert.Equal(t, w.Body.String(), "Internal error occured while disconnecting\n")
	})
}
