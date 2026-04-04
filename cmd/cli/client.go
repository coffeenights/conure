package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(cfg *Config) *apiClient {
	return &apiClient{
		baseURL: strings.TrimRight(cfg.Server, "/"),
		token:   cfg.Token,
		http:    &http.Client{},
	}
}

func (c *apiClient) do(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth", Value: c.token})

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

func (c *apiClient) get(path string) ([]byte, error) {
	data, _, err := c.do(http.MethodGet, path, nil)
	return data, err
}

func (c *apiClient) post(path string, body interface{}) ([]byte, error) {
	data, _, err := c.do(http.MethodPost, path, body)
	return data, err
}

func (c *apiClient) put(path string, body interface{}) ([]byte, error) {
	data, _, err := c.do(http.MethodPut, path, body)
	return data, err
}
