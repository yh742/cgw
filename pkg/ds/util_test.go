package ds

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"gotest.tools/assert"
)

func TestURLJoin(t *testing.T) {
	t.Run("success_case", func(t *testing.T) {
		testTable := map[string][2]string{
			"http://localhost:8080/ds/v1/refresh":  {"http://localhost:8080", "ds/v1/refresh"},
			"http://localhost:8080/ds/v1/refresh/": {"http://localhost:8080", "ds/v1/refresh/"},
		}
		for k, v := range testTable {
			joined, err := URLJoin(v[0], v[1])
			assert.NilError(t, err, k)
			assert.Equal(t, k, joined)
		}
	})

	t.Run("fail_case", func(t *testing.T) {
		testTable := map[string][2]string{
			"localhost:8080/ds/v1/refresh/": {"localhost:8080", "ds/v1/refresh/"},
		}
		for _, v := range testTable {
			_, err := URLJoin(v[0], v[1])
			assert.Error(t, err, "url format is incorrect, must specify protocol/scheme")
		}
	})

}

func TestHTTPRequest(t *testing.T) {
	jBytes, err := json.Marshal(map[string]string{"body": "test"})
	assert.NilError(t, err)
	byteReadCloser := ioutil.NopCloser(bytes.NewReader(jBytes))

	data, err := HTTPRequest("POST", "http://localhost:"+sm.port,
		map[string]string{"Authorization": "Bearer abcdefg"}, map[string]string{"query": "test"}, byteReadCloser, http.StatusOK)
	assert.NilError(t, err)

	// test request header
	assert.Equal(t, sm.GetTail(1).header.Get("Authorization"), "Bearer abcdefg")

	// test output of server
	assert.Equal(t, string(data), "test")

	// test request body store of server
	reqVal := &map[string]string{}
	err = json.Unmarshal(sm.GetTail(1).body, reqVal)
	assert.NilError(t, err)
	assert.Equal(t, (*reqVal)["body"], "test")
}
