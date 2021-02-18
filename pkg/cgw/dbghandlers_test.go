package cgw

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-redis/redis/v8"
	"gotest.tools/assert"
)

func TestDebugFlush(t *testing.T) {
	handler := flushHandler(gw.kv)
	redMock.ExpectFlushAll().SetErr(redis.Nil)
	w := &httptest.ResponseRecorder{}
	req := createTestRequest(t, nil, nil)
	handler(w, req)
}

func TestDebugSetToken(t *testing.T) {
	fakeToken := "1111.1111"
	handler := setTokenHandler(gw.SetToken)
	w := &httptest.ResponseRecorder{}
	req := createTestRequest(t, nil, nil)
	q := req.URL.Query()
	q.Add("token", fakeToken)
	req.URL.RawQuery = q.Encode()
	handler(w, req)
	assert.Equal(t, gw.token, fakeToken)
}

func TestDebugSetMEC(t *testing.T) {
	fakeMEC := "192.168.0.1"
	handler := setMECHandler(gw.SetMEC)
	w := &httptest.ResponseRecorder{}
	req := createTestRequest(t, nil, nil)
	q := req.URL.Query()
	q.Add("mec", fakeMEC)
	req.URL.RawQuery = q.Encode()
	handler(w, req)
	assert.Equal(t, gw.mecID, fakeMEC)
}

func TestAppendRequestLogs(t *testing.T) {
	entHandler := jsonDecodeHandler(EntityTokenReq, func(w http.ResponseWriter, req *http.Request) {}, gw.AppendLog)
	entityTokenReq := EntityTokenRequest{
		Token: "test.token",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	}
	req := createTestRequest(t, entityTokenReq, nil)
	w := &httptest.ResponseRecorder{}
	entHandler(w, req)

	entry, ok := gw.requestLog[0].(map[string]interface{})
	assert.Assert(t, ok)
	blob := entry[""]
	decoded, ok := blob.(*EntityTokenRequest)
	assert.Assert(t, ok)
	assert.Equal(t, *decoded, entityTokenReq)
}

func TestGetRequestLogs(t *testing.T) {
	gw.requestLog = make([]interface{}, 0)
	entityTokenReq := EntityTokenRequest{
		Token: "test.token",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	}
	gw.AppendLog("/url", entityTokenReq)
	w := &httptest.ResponseRecorder{
		Body: &bytes.Buffer{},
	}
	req := createTestRequest(t, nil, nil)
	handler := getReqLogHandler(gw.GetLogs)
	handler(w, req)
	b, err := json.Marshal([]interface{}{
		map[string]interface{}{
			"/url": entityTokenReq,
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(w.Body.String()), strings.TrimSpace(string(b)))
}

func TestClearLogs(t *testing.T) {
	gw.requestLog = make([]interface{}, 0)
	entityTokenReq := EntityTokenRequest{
		Token: "test.token",
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
	}
	gw.AppendLog("/url", entityTokenReq)
	assert.Equal(t, len(gw.requestLog), 1)
	req := createTestRequest(t, nil, nil)
	w := &httptest.ResponseRecorder{}
	handler := delReqLogHandler(gw.ClearLogs)
	handler(w, req)
	assert.Equal(t, len(gw.requestLog), 0)
}
