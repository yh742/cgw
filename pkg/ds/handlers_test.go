package ds

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gotest.tools/assert"
)

var sm ServiceMocks

func TestMain(m *testing.M) {
	// setup servers for unit testing
	sm = ServiceMocks{}
	setEndpoints(Config{
		CRS: CRSSettings{
			Server:       "http://localhost:9090",
			GetToken:     "/crs/v1/token",
			UpdateToken:  "/crs/v1/refresh",
			Registration: "/crs/v1/registration",
		},
		CAAS: CAASSettings{
			Server:         "http://localhost:9090",
			DeleteEntityID: "/caas/v1/entity/delete",
		},
		UpstreamReasonCode: []ReasonCode{Idle, NotAuthorized},
	})
	stop := make(chan struct{})
	go sm.StartServer("9090", stop)
	exitVal := m.Run()
	stop <- struct{}{}
	os.Exit(exitVal)
}

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
	err := deleteEntityID("123", Idle)
	assert.NilError(t, err)
	assert.Equal(t, sm.GetTail(2).query, "/crs/v1/token?entityid=123")
	deleteReq := &DeleteEntityRequest{}
	err = json.Unmarshal(sm.GetTail(1).body, deleteReq)
	assert.NilError(t, err)
	assert.Equal(t, *deleteReq, DeleteEntityRequest{
		Entity: "veh",
		EntityTokenPair: EntityTokenPair{
			EntityID: "123",
			Token:    "123456.123456",
		},
	})

}

func TestRefreshHandler(t *testing.T) {
	inReq := EntityTokenPair{
		EntityID: "123",
		Token:    "abcdefg.abcdefg",
	}
	jBytes, err := json.Marshal(inReq)
	assert.NilError(t, err)
	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))
	w := httptest.NewRecorder()
	RefreshHandler(w, &http.Request{
		Body: byteReadCloser,
	})
	rr := &EntityTokenPair{}
	err = json.Unmarshal(sm.GetTail(1).body, rr)
	assert.NilError(t, err)
	assert.Equal(t, *rr, inReq)
}
