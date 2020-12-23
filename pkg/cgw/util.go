package cgw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func printFuncName() string {
	fpcs := make([]uintptr, 1)
	// skip 3 levels to get the caller
	n := runtime.Callers(3, fpcs)
	if n == 0 {
		return ""
	}
	caller := runtime.FuncForPC(fpcs[0] - 1)
	if caller == nil {
		return ""
	}
	// return the name of the function
	return caller.Name()
}

// DebugLog logs debug message with function name
func DebugLog(value string, args ...interface{}) {
	template := printFuncName() + ": " + value
	log.Debug().Msgf(template, args...)
}

// ErrorLog logs error messages with function name
func ErrorLog(value string, args ...interface{}) {
	template := printFuncName() + ": " + value
	log.Error().Msgf(template, args...)
}

// IsEmpty is shorthand for strings.TrimSpace
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// HyphenConcat joins two strings with hyphen
func HyphenConcat(s1 string, s2 string) string {
	return strings.Join([]string{s1, s2}, "-")
}

// URLJoin joins url paths
func URLJoin(base string, path string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if b.Host == "" {
		return "", errors.New("url format is incorrect, must specify protocol/scheme")
	}
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(u).String(), nil
}

// HTTPResponse represents response from HTTP
type HTTPResponse struct {
	body   []byte
	status int
}

// HTTPRequest makes http requests
func HTTPRequest(ctx context.Context, method string, endpoint string, header map[string]string, query map[string]string, body io.Reader) (HTTPResponse, error) {
	// create request client to get entity ID from CRS
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return HTTPResponse{}, err
	}
	if header != nil {
		for k, v := range header {
			req.Header.Add(k, v)
		}
	}
	queries := req.URL.Query()
	for k, v := range query {
		queries.Add(k, v)
	}
	req.URL.RawQuery = queries.Encode()
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return HTTPResponse{}, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ErrorLog("unable to read response body, %v", err)
		return HTTPResponse{}, errors.New("can't read response body")
	}
	return HTTPResponse{
		body:   data,
		status: resp.StatusCode,
	}, nil
}

// JSONDecodeRequest decodes JSON from http requests
func JSONDecodeRequest(w http.ResponseWriter, r *http.Request, bodySize int64, obj interface{}) error {
	value := r.Header.Get("Content-Type")
	if IsEmpty(value) || strings.ToLower(value) != "application/json" {
		errString := fmt.Sprintf("Content-Type header is not \"%s\"", "application/json")
		http.Error(w, errString, http.StatusUnsupportedMediaType)
		return errors.New(errString)
	}

	if r.Body == nil {
		errString := fmt.Sprintf("Request body does not have a value")
		http.Error(w, errString, http.StatusBadRequest)
		return errors.New(errString)
	}

	// limit the amount of data and disable unknown field
	r.Body = http.MaxBytesReader(w, r.Body, bodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// decode message
	err := dec.Decode(obj)
	if err != nil {
		var msg string
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		// Catch any syntax errors in the JSON and send an error message
		// which interpolates the location of the problem to make it
		// easier for the client to fix.
		case errors.As(err, &syntaxError):
			msg = fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			http.Error(w, msg, http.StatusBadRequest)

		// In some circumstances Decode() may also return an
		// io.ErrUnexpectedEOF error for syntax errors in the JSON. There
		// is an open issue regarding this at
		// https://github.com/golang/go/issues/25956.
		case errors.Is(err, io.ErrUnexpectedEOF):
			msg = fmt.Sprintf("Request body contains badly-formed JSON")
			http.Error(w, msg, http.StatusBadRequest)

		// Catch any type errors, like trying to assign a string in the
		// JSON request body to a int field in our Person struct. We can
		// interpolate the relevant field name and position into the error
		// message to make it easier for the client to fix.
		case errors.As(err, &unmarshalTypeError):
			msg = fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			http.Error(w, msg, http.StatusBadRequest)

		// Catch the error caused by extra unexpected fields in the request
		// body. We extract the field name from the error message and
		// interpolate it in our custom error message. There is an open
		// issue at https://github.com/golang/go/issues/29035 regarding
		// turning this into a sentinel error.
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg = fmt.Sprintf("Request body contains unknown field %s", fieldName)
			http.Error(w, msg, http.StatusBadRequest)

		// An io.EOF error is returned by Decode() if the request body is
		// empty.
		case errors.Is(err, io.EOF):
			msg = "Request body must not be empty"
			http.Error(w, msg, http.StatusBadRequest)

		// Catch the error caused by the request body being too large. Again
		// there is an open issue regarding turning this into a sentinel
		// error at https://github.com/golang/go/issues/30715.
		case err.Error() == "http: request body too large":
			msg = fmt.Sprintf("Request body must not be larger than %d byte(s)", bodySize)
			http.Error(w, msg, http.StatusRequestEntityTooLarge)

		// Otherwise default to logging the error and sending a 500 Internal
		// Server Error response.
		default:
			msg = err.Error()
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return errors.New(msg)
	}

	// Call decode again, using a pointer to an empty anonymous struct as
	// the destination. If the request body only contained a single JSON
	// object this will return an io.EOF error. So if we get anything else,
	// we know that there is additional data in the request body.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		http.Error(w, msg, http.StatusBadRequest)
		return errors.New(msg)
	}

	DebugLog("%+v", obj)

	return nil
}
