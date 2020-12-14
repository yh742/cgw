package ds

import (
	"testing"

	"gotest.tools/assert"
)

func TestConfigParse(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string]Config{
			"./test/config/authNone.yaml": {
				MECID:          "rkln",
				ReadTimeout:    1000,
				WriteTimeout:   1000,
				HandlerTimeout: 1000,
				MaxHeaderBytes: 1000,
				Port:           "9090",
				Redis: RedisSettings{
					Server:   "localhost:6379",
					AuthFile: "/etc/ds/auth",
				},
				CAAS: CAASSettings{
					Server:           "localhost:8989",
					CreateEndpoint:   "/token",
					ValidateEndpoint: "/validate",
					DeleteEndpoint:   "/entity/delete",
				},
				MQTT: MQTTSettings{
					Server:      "localhost:1883",
					AuthType:    NoAuth,
					SuccessCode: 0x03,
				},
			},
			"./test/config/authFile.yaml": {
				MECID:              "rkln",
				ReadTimeout:        1000,
				WriteTimeout:       1000,
				HandlerTimeout:     1000,
				MaxHeaderBytes:     1000,
				Port:               "9090",
				UpstreamReasonCode: []ReasonCode{0x98, 0x87},
				CAAS: CAASSettings{
					Server:           "localhost:8989",
					CreateEndpoint:   "/token",
					ValidateEndpoint: "/validate",
					DeleteEndpoint:   "/entity/delete",
				},
				MQTT: MQTTSettings{
					Server:      "localhost:1883",
					SuccessCode: 0x03,
					AuthType:    FileBased,
					AuthFile:    "/etc/ds/auth",
				},
				Redis: RedisSettings{
					Server:   "localhost:6379",
					AuthFile: "/etc/ds/auth",
				},
			},
			"./test/config/authCRS.yaml": {
				MECID:              "rkln",
				ReadTimeout:        1000,
				WriteTimeout:       1000,
				HandlerTimeout:     1000,
				MaxHeaderBytes:     1000,
				Port:               "9090",
				UpstreamReasonCode: []ReasonCode{0x98, 0x87},
				CAAS: CAASSettings{
					Server:           "localhost:8989",
					CreateEndpoint:   "/token",
					ValidateEndpoint: "/validate",
					DeleteEndpoint:   "/entity/delete",
				},
				MQTT: MQTTSettings{
					Server:      "localhost:1883",
					SuccessCode: 0x03,
					AuthType:    CRSBased,
					CRS: CRSSettings{
						Entity:               "sw",
						Server:               "vzmode-rkln.mec/registration:30413",
						CfgPath:              "/etc/ds/crs/cfg",
						TokenFile:            "/etc/ds/crs/token",
						RegistrationEndpoint: "/registration",
					},
				},
				Redis: RedisSettings{
					Server:   "localhost:6379",
					AuthFile: "/etc/ds/auth",
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
			"./test/config/missingServer.yaml":    "missing required server locations",
			"./test/config/missingEndpoint.yaml":  "missing required endpoint",
			"./test/config/missingCRS.yaml":       "missing required crs auth fields",
			"./test/config/missingRedisAuth.yaml": "missing required redis auth file",
		}
		for k, v := range testTable {
			cfg := &Config{}
			err := cfg.Parse(k)
			assert.Error(t, err, v)
		}
	})
}
