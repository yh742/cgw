package cgw

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestNewGateway(t *testing.T) {
	ds := &dsMock{}
	cgw, err := NewCAASGateway("./test/config/cgw.yaml", gw.kv, ds)
	assert.NilError(t, err)
	assert.Equal(t, cgw.port, "8080")
	assert.Equal(t, cgw.mecID, "rkln")
	assert.Equal(t, cgw.readTO, 1000*time.Millisecond)
	assert.Equal(t, cgw.writeTO, 5000*time.Millisecond)
	assert.Equal(t, cgw.handlerTO, 4000*time.Millisecond)
	assert.Equal(t, cgw.maxHeaderBytes, 1000)
	assert.Equal(t, cgw.token, "test.test")
	assert.Equal(t, cgw.caasCreateURL, "http://localhost:9090/caas/v1/token/entity")
	assert.Equal(t, cgw.caasDeleteEntityIDURL, "http://localhost:9090/caas/v1/token/entity/delete")
	assert.Equal(t, cgw.upstreamReasonCodes[0x98], true)
	assert.Equal(t, cgw.upstreamReasonCodes[0x87], true)
}

func TestStartServer(t *testing.T) {
	ds := &dsMock{}
	cgw, err := NewCAASGateway("./test/config/cgw.yaml", gw.kv, ds)
	assert.NilError(t, err)
	go cgw.StartServer()
	defer func() {
		sm.ClearDB()
		cgw.StopSignal <- struct{}{}
	}()
	t.Run("check_endpoints", func(t *testing.T) {
		etr := EntityTokenRequest{
			Token: "test.test",
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
		}
		jBytes, err := json.Marshal(etr)
		assert.NilError(t, err)

		// check create new token
		redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, cgw.handlerTO).SetVal(true)
		redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
		defer redMock.ClearExpect()
		resp, err := http.Post("http://localhost:8080/cgw/v1/token", "application/json", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)

		// validate credentials
		redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, cgw.handlerTO).SetVal(true)
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		defer redMock.ClearExpect()
		resp, err = http.Post("http://localhost:8080/cgw/v1/token/validate", "application/json", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)

		// // refresh credentials
		redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, cgw.handlerTO).SetVal(true)
		redMock.ExpectExists("veh-1234").SetVal(1)
		redMock.ExpectSet("veh-1234", "test.test", 0).SetVal("")
		defer redMock.ClearExpect()
		resp, err = http.Post("http://localhost:8080/cgw/v1/token/refresh", "application/json", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)

		// disconnect a client
		dr := DisconnectRequest{
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
			ReasonCode: Reauthenticate,
			NextServer: "localhost:8080",
		}
		jBytes, err = json.Marshal(dr)
		assert.NilError(t, err)
		redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, cgw.handlerTO).SetVal(true)
		redMock.ExpectGet("veh-1234").SetVal("test.test")
		redMock.ExpectDel("veh-1234").SetVal(1)
		defer redMock.ClearExpect()
		resp, err = http.Post("http://localhost:8080/cgw/v1/disconnect", "application/json", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
	})

	t.Run("timeout", func(t *testing.T) {
		etr := &EntityTokenRequest{
			Token: "sleep.test",
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
		}
		redMock.Regexp().ExpectSetNX("lock:veh-1234", `[a-z1-9]*`, cgw.handlerTO).SetVal(true)
		defer redMock.ClearExpect()
		jBytes, err := json.Marshal(etr)
		assert.NilError(t, err)
		resp, err := http.Post("http://localhost:8080/cgw/v1/token", "application/json", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)
		bytes, err := ioutil.ReadAll(resp.Body)
		assert.NilError(t, err)
		assert.Equal(t, string(bytes), "Timed out processing request")
	})
}
