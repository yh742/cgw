package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestConfigParse(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string]Config{
			"./test/config/noAuth.yaml": {
				Port:     "9090",
				AuthType: 0,
				CAAS: CAASSettings{
					Server: "localhost:8989",
				},
				CRS: CRSSettings{
					Server: "localhost:8080",
				},
				MQTT: MQTTSettings{
					Server:      "localhost:1883",
					SuccessCode: 0x03,
				},
			},
			"./test/config/fileAuth.yaml": {
				Port:               "9090",
				AuthType:           1,
				UpstreamReasonCode: []ReasonCode{0x98, 0x87},
				CRS: CRSSettings{
					Server:      "localhost:8080",
					GetToken:    "/token",
					UpdateToken: "/token",
				},
				CAAS: CAASSettings{
					Server:         "localhost:8989",
					DeleteEntityID: "/delete/entity",
				},
				MQTT: MQTTSettings{
					Server:      "localhost:1883",
					SuccessCode: 0x03,
					AuthFile:    "/etc/ds/auth",
				},
			},
			"./test/config/crsAuth.yaml": {
				Port:     "8080",
				AuthType: 2,
				MQTT: MQTTSettings{
					Server:      "localhost:1883",
					SuccessCode: 0x03,
					AuthFile:    "/etc/ds/auth",
				},
				CAAS: CAASSettings{
					Server:         "localhost:8989",
					DeleteEntityID: "/delete/entity",
				},
				CRS: CRSSettings{
					Entity:      "sw",
					Server:      "vzmode-rkln.mec/registration:30413",
					CfgPath:     "/etc/ds/crs/cfg",
					TokenFile:   "/etc/ds/crs/token",
					GetToken:    "/token",
					UpdateToken: "/token",
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
			"./test/config/missingReq.yaml":  "missing required value",
			"./test/config/missingAuth.yaml": "missing mqtt auth file",
			"./test/config/missingCRS.yaml":  "missing required CRS value",
		}
		for k, v := range testTable {
			cfg := &Config{}
			err := cfg.Parse(k)
			assert.Error(t, err, v)
		}
	})
}
