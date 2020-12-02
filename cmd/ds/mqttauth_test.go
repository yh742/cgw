package main

import (
	"net/http"
	"testing"

	"gotest.tools/assert"
)

func TestCRSCredentials(t *testing.T) {
	cfg := Config{
		CRS: CRSSettings{
			Entity:    "sw",
			Server:    "http://localhost:8085/registration",
			CfgPath:   "./test/auth/crsFake",
			TokenFile: "./test/auth/crsFake",
		},
	}
	stop := make(chan struct{})
	ss := StubServer{}
	go ss.RunStubServer("8085", "POST", "/registration", map[string]string{
		"ID": "12",
	}, http.StatusCreated, stop)
	defer func() {
		stop <- struct{}{}
	}()
	mAuth, err := CRSCredentials(cfg)
	assert.NilError(t, err)
	assert.Equal(t, mAuth.user, "sw-12")
	assert.Equal(t, mAuth.password, "password")
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
