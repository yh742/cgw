package ds

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

	data, err := HTTPRequest(context.Background(), "POST", "http://localhost:"+sm.port,
		map[string]string{"Authorization": "Bearer abcdefg"}, map[string]string{"query": "test"}, byteReadCloser)
	assert.NilError(t, err)

	// test request header
	assert.Equal(t, sm.GetTail(1).header.Get("Authorization"), "Bearer abcdefg")

	// test output of server
	assert.Equal(t, data.status, http.StatusOK)
	assert.Equal(t, string(data.body), "test")

	// test request body store of server
	reqVal := &map[string]string{}
	err = json.Unmarshal(sm.GetTail(1).body, reqVal)
	assert.NilError(t, err)
	assert.Equal(t, (*reqVal)["body"], "test")
}

func TestIsEmpty(t *testing.T) {
	assert.Assert(t, IsEmpty(""))
	assert.Assert(t, IsEmpty("   	"))
	assert.Assert(t, !IsEmpty("sdfdsf"))
}

type testStruct struct {
	Field1 string `json:"field1"`
	Field2 string `json:"field2"`
}

func (tokReq *testStruct) FieldsEmpty() bool {
	return false
}

func TestJSONDecodeRequest(t *testing.T) {

	t.Run("success_case", func(t *testing.T) {
		// create new writer
		w := httptest.NewRecorder()

		// create new request
		reqJSON := testStruct{
			Field1: "value1",
			Field2: "value2",
		}
		jBytes, err := json.Marshal(reqJSON)
		assert.NilError(t, err)
		req, err := http.NewRequest("POST", "http://localhost:9090", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		req.Header.Add("Content-Type", "application/json")

		// read request back
		reqBack := &testStruct{}
		err = JSONDecodeRequest(w, req, 1<<12, reqBack)
		assert.NilError(t, err)
		assert.Equal(t, reqJSON, *reqBack)
	})

	t.Run("fail_case", func(t *testing.T) {
		testTable := map[string]*http.Request{}

		req, err := http.NewRequest("POST", "http://localhost:9090", nil)
		assert.NilError(t, err)
		req.Header.Add("content-type", "applicationjson")
		testTable["Content-Type header is not \"application/json\""] = req

		req, err = http.NewRequest("POST", "http://localhost:9090", bytes.NewBuffer([]byte("{sdfsd:")))
		assert.NilError(t, err)
		req.Header.Add("content-type", "application/json")
		testTable["Request body contains badly-formed JSON (at position 2)"] = req

		jsonMap := map[string]string{
			"Field1": "123",
			"Field2": "456",
			"Field3": "789",
		}
		jBytes, err := json.Marshal(jsonMap)
		assert.NilError(t, err)
		req, err = http.NewRequest("POST", "http://localhost:9090", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		req.Header.Add("content-type", "application/json")
		testTable["Request body contains unknown field \"Field3\""] = req

		jsonMap = map[string]string{
			"Field1": "123",
			"Field2": "456",
		}
		jBytes, err = json.Marshal(jsonMap)
		assert.NilError(t, err)
		req, err = http.NewRequest("POST", "http://localhost:9090", bytes.NewBuffer(jBytes))
		assert.NilError(t, err)
		req.Header.Add("content-type", "application/json")
		testTable["Request body must not be larger than 1 byte(s)"] = req

		req, err = http.NewRequest("POST", "http://localhost:9090", bytes.NewBuffer([]byte("")))
		assert.NilError(t, err)
		req.Header.Add("content-type", "application/json")
		testTable["Request body must not be empty"] = req

		req, err = http.NewRequest("POST", "http://localhost:9090", nil)
		assert.NilError(t, err)
		req.Header.Add("content-type", "application/json")
		testTable["Request body does not have a value"] = req

		for k, v := range testTable {
			w := httptest.NewRecorder()
			reqBack := &testStruct{}
			var err error
			if k == "Request body must not be larger than 1 byte(s)" {
				err = JSONDecodeRequest(w, v, 1, reqBack)
			} else {
				err = JSONDecodeRequest(w, v, 1<<12, reqBack)
			}
			assert.Error(t, err, k)
		}
	})

}
