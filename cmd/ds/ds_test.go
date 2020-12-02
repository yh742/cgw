package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

type dsMock struct {
	DisconnectRequest
}

func (m *dsMock) Disconnect(ds DisconnectRequest, w http.ResponseWriter) {
	m.DisconnectRequest = ds
	return
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

func TestDeleteEntityID(t *testing.T) {
	downstreamReasonCodes = map[ReasonCode]bool{
		Idle:          true,
		NotAuthorized: true,
	}
	getTokenEndpoint = "http://localhost:8080/token"
	deleteEntityIDEndpoint = "http://localhost:9090/delete"
	stop := make(chan struct{})
	stop2 := make(chan struct{})
	tokenStore := StubServer{}
	caas := StubServer{}
	go tokenStore.RunStubServer("8080", "GET", "/token", map[string]string{"token": "1sdfh3h2.2189r"}, http.StatusOK, stop)
	go caas.RunStubServer("9090", "POST", "/delete", nil, http.StatusOK, stop2)
	defer func() {
		stop <- struct{}{}
		stop2 <- struct{}{}
	}()
	err := DeleteEntityID("123", Idle)
	assert.NilError(t, err)
	assert.Equal(t, tokenStore.reqQueryHistory[0].String(), "/token?entityid=123")
	deleteReq := &DeleteEntityRequest{}
	err = json.Unmarshal(caas.reqHistory[0], deleteReq)
	assert.NilError(t, err)
	assert.Equal(t, *deleteReq, DeleteEntityRequest{
		Entity: "veh",
		RefreshRequest: RefreshRequest{
			EntityID: "123",
			Token:    "1sdfh3h2.2189r",
		},
	})

}

func TestRefreshHandler(t *testing.T) {
	updateTokenEndpoint = "http://localhost:8081/update"
	stop := make(chan struct{})
	ss := StubServer{}
	inReq := RefreshRequest{
		EntityID: "123",
		Token:    "abcdefg.abcdefg",
	}
	go ss.RunStubServer("8081", "POST", "/update", nil, http.StatusOK, stop)
	defer func() {
		stop <- struct{}{}
	}()
	jBytes, err := json.Marshal(inReq)
	assert.NilError(t, err)
	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))
	w := httptest.NewRecorder()
	RefreshHandler(w, &http.Request{
		Body: byteReadCloser,
	})
	assert.Equal(t, len(ss.reqHistory), 1)
	assert.NilError(t, err)
	rr := &RefreshRequest{}
	err = json.Unmarshal(ss.reqHistory[0], rr)
	assert.NilError(t, err)
	assert.Equal(t, *rr, inReq)
}
