package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

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

// HTTPRequest makes http requests
func HTTPRequest(method string, endpoint string, query map[string]string, body io.Reader, status int) ([]byte, error) {
	// create request client to get entity ID from CRS
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	queries := req.URL.Query()
	for k, v := range query {
		queries.Add(k, v)
	}
	req.URL.RawQuery = queries.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		log.Error().Msgf("responded with status code, %d", resp.StatusCode)
		return nil, errors.New("responded with failure code")
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().Msgf("unable to read response body, %v", err)
		return nil, errors.New("can't read response body")
	}
	return data, nil
}
