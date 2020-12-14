package ds

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
	"gotest.tools/assert"
)

type dsMock struct {
	DisconnectRequest
}

func (m *dsMock) Disconnect(ds DisconnectRequest, w http.ResponseWriter) {
	m.DisconnectRequest = ds
	return
}

type redisMock struct {
	rCli *redis.Client
}

func (kv redisMock) Get(ctx context.Context, key string) (string, error) {
	return kv.rCli.Get(ctx, key).Result()
}

func (kv redisMock) Set(ctx context.Context, key string, value string) error {
	return kv.rCli.Set(ctx, key, value, 0).Err()
}

func (kv redisMock) SetIfNotExist(ctx context.Context, key string, value string) (bool, error) {
	return kv.rCli.SetNX(ctx, key, value, 0).Result()
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
		kv: redisMock{
			rCli: rClient,
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

func TestValidateToken(t *testing.T) {
	httpHandler := http.TimeoutHandler(validateToken(gw.kv, gw.caasValidateURL, gw.mecID), time.Second, "Timed out")
	// common structs
	valStruct := ValidateTokenRequest{
		EntityTokenRequest: EntityTokenRequest{
			Entity: "veh",
			Token:  "test.test",
			EntityIDStruct: EntityIDStruct{
				"1234",
			},
		},
		MEC: gw.mecID,
	}

	redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
	// pre-populated a entry in redis cache
	gw.kv.Set(context.Background(), "veh-1234", "test.test")

	t.Run("verify_through_cached", func(t *testing.T) {
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		writer := httptest.NewRecorder()
		req := createTestRequest(t, valStruct.EntityTokenRequest)
		httpHandler.ServeHTTP(writer, req)
		assert.Equal(t, writer.Code, http.StatusOK)
	})

	// t.Run("verify_through_caas", func(t *testing.T) {
	// 	writer := httptest.NewRecorder()
	// 	redMock.ExpectGet("veh-4321").RedisNil()
	// 	redMock.ExpectSet("veh-4321", "test.test", 0).SetVal("")
	// 	valStruct.EntityTokenRequest.EntityID = "4321"
	// 	req := createTestRequest(t, valStruct.EntityTokenRequest)
	// 	httpHandler.ServeHTTP(writer, req)
	// 	assert.Equal(t, writer.Code, http.StatusOK)
	// })

	t.Run("fail_case", func(t *testing.T) {
		writer := httptest.NewRecorder()
		redMock.ExpectGet("veh-4321").RedisNil()
		// redMock.ExpectSet("veh-4321", "test.test", 0).SetVal("")
		valStruct.EntityTokenRequest.Token = "fail.test"
		req := createTestRequest(t, valStruct.EntityTokenRequest)
		httpHandler.ServeHTTP(writer, req)
		assert.Equal(t, writer.Code, http.StatusForbidden)
	})

}

func TestCreateNewToken(t *testing.T) {
	// setup http handler
	httpHandler := http.TimeoutHandler(createNewToken(gw.kv, gw.caasCreateURL, gw.mecID), time.Second, "Timed out")
	// common structs
	valStruct := ValidateTokenRequest{
		EntityTokenRequest: EntityTokenRequest{
			Entity: "veh",
			Token:  "test.test",
			EntityIDStruct: EntityIDStruct{
				"1234",
			},
		},
		MEC: gw.mecID,
	}

	// test cases:
	// (1) run success case
	// (2) run conflict case
	// (3) run http timeout case
	// (4) run fail case
	t.Run("success_case", func(t *testing.T) {
		// set expectations
		redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")

		// create request and run
		writer := httptest.NewRecorder()
		req := createTestRequest(t, valStruct.EntityTokenRequest)
		httpHandler.ServeHTTP(writer, req)

		// check:
		// (1) if caas got the correct request
		// (2) if caas responded with OK and set KV
		// (3) if returned OK status
		jsBytes, err := json.Marshal(valStruct)
		assert.NilError(t, err)
		assert.Equal(t, writer.Result().StatusCode, http.StatusOK)
		assert.Equal(t, string(sm.GetTail(1).body), string(jsBytes))
	})

	t.Run("conflict_case", func(t *testing.T) {
		// set expectations
		redMock.ExpectSet("veh-4321", "repeated.test", 0).SetVal("")

		// create request and run
		writer := httptest.NewRecorder()
		valStruct.EntityTokenRequest.Token = "repeated.test"
		req := createTestRequest(t, valStruct.EntityTokenRequest)
		httpHandler.ServeHTTP(writer, req)

		// check:
		// (1) 409 conflict is received
		// (2) bytes unmarshalled to json properly
		jsBytes, err := json.Marshal(valStruct)
		assert.NilError(t, err)
		assert.Equal(t, string(sm.GetTail(1).body), string(jsBytes))
		assert.Equal(t, writer.Result().StatusCode, http.StatusConflict)
	})

	t.Run("fail_case_timeout", func(t *testing.T) {
		// create request and run
		writer := httptest.NewRecorder()
		valStruct.EntityTokenRequest.Token = "sleep.test"
		req := createTestRequest(t, valStruct.EntityTokenRequest)
		httpHandler.ServeHTTP(writer, req)

		// check if timed out error is throw (from timeout handler)
		assert.Equal(t, writer.Result().StatusCode, http.StatusServiceUnavailable)
		assert.Equal(t, string(writer.Body.Bytes()), "Timed out")
	})

	t.Run("fail_case", func(t *testing.T) {
		// create fail request and run
		writer := httptest.NewRecorder()
		valStruct.EntityTokenRequest.Token = "fail.test"
		req := createTestRequest(t, valStruct.EntityTokenRequest)
		httpHandler.ServeHTTP(writer, req)

		// check:
		// (1) 409 conflict is received
		// (2) bytes unmarshalled to json properly
		jsBytes, err := json.Marshal(valStruct)
		assert.NilError(t, err)
		assert.Equal(t, string(sm.GetTail(1).body), string(jsBytes))
		assert.Equal(t, writer.Result().StatusCode, http.StatusBadRequest)
	})
}

func TestDisconnectHandler(t *testing.T) {
	ds := &dsMock{}
	jBytes, err := json.Marshal(DisconnectRequest{
		Entity:     "sw",
		EntityID:   "12",
		ReasonCode: 0x9C,
		NextServer: "localhost:8080",
	})
	assert.NilError(t, err)
	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))
	handler := DisconnectHandler(ds)
	handler(nil, &http.Request{
		Body: byteReadCloser,
	})
	assert.Equal(t, ds.Entity, "sw")
	assert.Equal(t, ds.EntityID, "12")
	assert.Equal(t, ds.NextServer, "localhost:8080")
	assert.Equal(t, ds.ReasonCode, ReasonCode(0x9C))
}

// func TestDeleteEntityID(t *testing.T) {
// 	err := deleteEntityID("123", Idle)
// 	assert.NilError(t, err)
// 	assert.Equal(t, sm.GetTail(2).query, "/crs/v1/token?entityid=123")
// 	deleteReq := &DeleteEntityRequest{}
// 	err = json.Unmarshal(sm.GetTail(1).body, deleteReq)
// 	assert.NilError(t, err)
// 	assert.Equal(t, *deleteReq, DeleteEntityRequest{
// 		Entity: "veh",
// 		EntityTokenPair: EntityTokenPair{
// 			EntityID: "123",
// 			Token:    "123456.123456",
// 		},
// 	})

// }

// func TestRefreshHandler(t *testing.T) {
// 	inReq := EntityTokenPair{
// 		EntityID: "123",
// 		Token:    "abcdefg.abcdefg",
// 	}
// 	jBytes, err := json.Marshal(inReq)
// 	assert.NilError(t, err)
// 	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))
// 	w := httptest.NewRecorder()
// 	RefreshHandler(w, &http.Request{
// 		Body: byteReadCloser,
// 	})
// 	rr := &EntityTokenPair{}
// 	err = json.Unmarshal(sm.GetTail(1).body, rr)
// 	assert.NilError(t, err)
// 	assert.Equal(t, *rr, inReq)
// }
