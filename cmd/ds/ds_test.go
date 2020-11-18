package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"gotest.tools/assert"
)

func TestConfigParse(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string]Config{
			"./test/config/noAuth.yaml": {
				Port: "9090",
				MQTT: MQTTSettings{
					Server:      "localhost",
					Port:        "1883",
					SuccessCode: 0x03,
				},
			},
			"./test/config/fileAuth.yaml": {
				Port: "9090",
				MQTT: MQTTSettings{
					Server:      "localhost",
					Port:        "1883",
					SuccessCode: 0x03,
					AuthFile:    "/etc/ds/auth",
				},
			},
			"./test/config/crsAuth.yaml": {
				Port: "8080",
				MQTT: MQTTSettings{
					Server:      "localhost",
					Port:        "1883",
					SuccessCode: 0x03,
					AuthFile:    "/etc/ds/auth",
				},
				CRS: CRSSettings{
					Entity:    "sw",
					Server:    "vzmode-rkln.mec/registration:30413",
					CfgPath:   "/etc/ds/crs/cfg",
					TokenFile: "/etc/ds/crs/token",
				},
			},
		}
		for k, v := range testTable {
			cfg := &Config{}
			err := cfg.Parse(k)
			assert.NilError(t, err)
			assert.DeepEqual(t, *cfg, v)
		}
	})

	t.Run("fail_case", func(t *testing.T) {
		testTable := map[string]string{
			"./test/config/missingReq.yaml": "missing required value",
			"./test/config/missingCRS.yaml": "missing required CRS value",
		}
		for k, v := range testTable {
			cfg := &Config{}
			err := cfg.Parse(k)
			assert.Error(t, err, v)
		}
	})
}

func TestFileCredentials(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		cfg := Config{
			MQTT: MQTTSettings{
				AuthFile: "./test/auth/authFile",
			},
		}
		mAuth, err := FileCredentials(cfg)
		assert.NilError(t, err)
		assert.Equal(t, mAuth.user, "user")
		assert.Equal(t, mAuth.password, "password")
	})
	t.Run("fail_case", func(t *testing.T) {
		cfg := Config{
			MQTT: MQTTSettings{
				AuthFile: "./test/auth/badAuthFile",
			},
		}
		_, err := FileCredentials(cfg)
		assert.Error(t, err, "can't read password from auth file")
	})
}

func CRSStubServer(stop <-chan struct{}) {
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    "localhost:8080",
		Handler: router,
	}
	router.HandleFunc("/registration", func(w http.ResponseWriter, r *http.Request) {
		mapD := map[string]string{"ID": "12"}
		mapB, _ := json.Marshal(mapD)
		w.WriteHeader(http.StatusCreated)
		w.Write(mapB)
	})
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()
	<-stop
	wait := time.Second * 10
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	srv.Shutdown(ctx)
}

func TestCRSCredentials(t *testing.T) {
	cfg := Config{
		CRS: CRSSettings{
			Entity:    "sw",
			Server:    "http://localhost:8080/registration",
			CfgPath:   "./test/auth/crsFake",
			TokenFile: "./test/auth/crsFake",
		},
	}
	stop := make(chan struct{})
	go CRSStubServer(stop)
	defer func() {
		stop <- struct{}{}
	}()
	mAuth, err := CRSCredentials(cfg)
	assert.NilError(t, err)
	assert.Equal(t, mAuth.user, "sw-12")
	assert.Equal(t, mAuth.password, "password")
}

func TestNewMQTTDisconnector(t *testing.T) {
	testTable := map[MQTTAuth]Config{
		{
			"",
			"",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost",
				Port:        "1883",
				SuccessCode: 0x03,
			},
		},
		{
			"user",
			"password",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost",
				Port:        "1883",
				SuccessCode: 0x03,
				AuthFile:    "./test/auth/authFile",
			},
		},
		{
			"user",
			"password",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost",
				Port:        "1883",
				SuccessCode: 0x03,
				AuthFile:    "./test/auth/authFile",
			},
		},
		{
			"sw-12",
			"password",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost",
				Port:        "1883",
				SuccessCode: 0x03,
				AuthFile:    "./test/auth/authFile",
			},
			CRS: CRSSettings{
				Entity:    "sw",
				Server:    "http://localhost:8080/registration",
				CfgPath:   "./test/auth/crsFake",
				TokenFile: "./test/auth/crsFake",
			},
		},
	}
	stop := make(chan struct{})
	go CRSStubServer(stop)
	defer func() {
		stop <- struct{}{}
	}()
	for k, v := range testTable {
		disconnecter, err := NewMQTTDisconnecter(v)
		assert.NilError(t, err)
		mDS, ok := disconnecter.(*MQTTDisconnecter)
		assert.Assert(t, ok)
		assert.Equal(t, mDS.SuccessCode, byte(3))
		assert.Equal(t, mDS.ConnOpts.Username, k.user)
		assert.Equal(t, mDS.ConnOpts.Password, k.password)
		assert.Equal(t, mDS.ConnOpts.Servers[0].Host, "localhost:1883")
	}
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
	jByte, err := json.Marshal(DisconnectRequest{
		Entity:     "sw",
		EntityID:   "12",
		ReasonCode: 0x9C,
		NextServer: "localhost:8080",
	})
	assert.NilError(t, err)
	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jByte))
	handler := DisconnectHandler(ds)
	handler(nil, &http.Request{
		Body: byteReadCloser,
	})
	assert.Equal(t, ds.Entity, "sw")
	assert.Equal(t, ds.EntityID, "12")
	assert.Equal(t, ds.NextServer, "localhost:8080")
	assert.Equal(t, ds.ReasonCode, byte(0x9C))
}

func TestBuildClientID(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string]DisconnectRequest{
			"sw-1232-156-rocklin.mec": {
				Entity:     "sw",
				EntityID:   "1232",
				ReasonCode: 0x9C,
				NextServer: "rocklin.mec",
			},
			"admin-12-152": {
				Entity:     "admin",
				EntityID:   "12",
				ReasonCode: 0x98,
				NextServer: " ",
			},
		}
		for k, v := range testTable {
			ID, err := buildClientID(v)
			assert.NilError(t, err)
			assert.Equal(t, ID, k)
		}
	})
	t.Run("fail_case", func(t *testing.T) {
		testTable := map[string]DisconnectRequest{
			"entity type is not supported": {
				Entity:   "",
				EntityID: "123",
			},
			"entity ID is empty": {
				Entity:   "sw",
				EntityID: "  ",
			},
			"reason code is not valid": {
				Entity:     "sw",
				EntityID:   "134",
				ReasonCode: 0xF2,
			},
		}
		for k, v := range testTable {
			_, err := buildClientID(v)
			assert.Error(t, err, k)
		}
	})
}
