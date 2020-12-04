package ds

import (
	"testing"

	"gotest.tools/assert"
)

func TestCRSCredentials(t *testing.T) {
	mAuth, err := CRSCredentials("sw", "./test/auth/crsFake", "./test/config/crsCfg.json")
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
