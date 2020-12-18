package ds

import (
	"testing"

	"gotest.tools/assert"
)

func TestBuildClientID(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string]DisconnectRequest{
			"sw-1232-156-rocklin.mec": {
				EntityPair: EntityPair{
					Entity:   "sw",
					EntityID: "1232",
				},
				ReasonCode: 0x9C,
				NextServer: "rocklin.mec",
			},
			"admin-12-152": {
				EntityPair: EntityPair{
					Entity:   "admin",
					EntityID: "12",
				},
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
				EntityPair: EntityPair{
					Entity:   "",
					EntityID: "123",
				},
			},
			"entity ID is empty": {
				EntityPair: EntityPair{
					Entity:   "sw",
					EntityID: "  ",
				},
			},
			"reason code is not valid": {
				EntityPair: EntityPair{
					Entity:   "sw",
					EntityID: "134",
				},
				ReasonCode: 0xF2,
			},
		}
		for k, v := range testTable {
			_, err := buildClientID(v)
			assert.Error(t, err, k)
		}
	})
}

func TestNewMQTTDisconnector(t *testing.T) {
	testTable := map[UserPassword]Config{
		{
			"",
			"",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost:1883",
				SuccessCode: 0x03,
			},
		},
		{
			"user",
			"password",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost:1883",
				SuccessCode: 0x03,
				AuthType:    FileBased,
				AuthFile:    "./test/auth/authFile",
			},
		},
		{
			"user",
			"password",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost:1883",
				SuccessCode: 0x03,
				AuthType:    FileBased,
				AuthFile:    "./test/auth/authFile",
			},
		},
		{
			"sw-12",
			"password",
		}: {
			Port: "8080",
			MQTT: MQTTSettings{
				Server:      "localhost:1883",
				SuccessCode: 0x03,
				AuthType:    CRSBased,
				CRS: CRSSettings{
					Entity:    "sw",
					Server:    "http://localhost:9090/crs/v1/registration",
					CfgPath:   "./test/config/crsCfg.json",
					TokenFile: "./test/auth/crsFake",
				},
			},
		},
	}
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
