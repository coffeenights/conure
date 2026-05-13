// Package apiclient is the typed HTTP client for the Conure API as used by
// the CLI. It owns request/response plumbing, error parsing, and one method
// per endpoint. Commands depend on this package; this package depends only
// on pkg/api (shared DTOs) and the standard library.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is the per-process Conure API client. Construct with New. The zero
// value is not valid — callers must supply at least a base URL.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client that talks to baseURL using token as a cookie auth
// header. baseURL may include a trailing slash; it will be trimmed.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{},
	}
}

// BaseURL is exposed so commands can echo it back to the user ("logged in
// to <baseURL>"). Not used internally.
func (c *Client) BaseURL() string { return c.baseURL }

// do issues a request and returns the response body. Non-2xx becomes an
// error carrying the parsed server message; callers don't need to inspect
// the body themselves on the error path.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.AddCookie(&http.Cookie{Name: "auth", Value: c.token})
	}

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
		return respBody, resp.StatusCode, fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, ParseServerError(respBody))
	}
	return respBody, resp.StatusCode, nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	data, _, err := c.do(ctx, http.MethodGet, path, nil)
	return data, err
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.do(ctx, http.MethodPost, path, body)
	return data, err
}

func (c *Client) put(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.do(ctx, http.MethodPut, path, body)
	return data, err
}

// Stream issues a GET and hands the live response body back to the caller.
// Caller MUST close the returned ReadCloser. The context cancels both the
// in-flight request and the body read, so Ctrl-C in a `conure logs -f`
// cleanly tears the connection down instead of dropping mid-line.
func (c *Client) Stream(ctx context.Context, path string, query url.Values) (io.ReadCloser, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.AddCookie(&http.Cookie{Name: "auth", Value: c.token})
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, ParseServerError(body))
	}
	return resp.Body, nil
}
