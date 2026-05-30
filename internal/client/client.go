// Package client is a thin HTTP client for the uapi REST API.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxLockRetries bounds how many times a write is retried when the server
// reports 423 locked (the global transaction flock is held by another writer).
const maxLockRetries = 5

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// insecure skips TLS verification, needed against uapi's default self-signed certificate.
func New(baseURL, token string, insecure bool) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 60 * time.Second, Transport: transport},
	}
}

type APIError struct {
	Status  int
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Errors  []FieldError `json:"errors"`
}

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "uapi %d %s: %s", e.Status, e.Code, e.Message)
	for _, fe := range e.Errors {
		fmt.Fprintf(&b, "\n  - %s: %s (%s)", fe.Field, fe.Message, fe.Code)
	}
	return b.String()
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("encoding request body: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, err
		}
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusLocked && attempt < maxLockRetries {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			select {
			case <-ctx.Done():
				return nil, resp.StatusCode, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, resp.StatusCode, nil
		}

		return nil, resp.StatusCode, decodeError(resp.StatusCode, respBody)
	}
}

func retryAfter(header string) time.Duration {
	if header != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Second
}

func decodeError(status int, body []byte) error {
	apiErr := &APIError{Status: status}
	if len(body) > 0 && json.Unmarshal(body, apiErr) == nil && apiErr.Code != "" {
		return apiErr
	}
	apiErr.Code = "unknown"
	apiErr.Message = strings.TrimSpace(string(body))
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(status)
	}
	return apiErr
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusNotFound
	}
	return false
}

func (c *Client) GetObject(ctx context.Context, path string) (obj map[string]any, found bool, err error) {
	raw, _, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		if IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false, fmt.Errorf("decoding response: %w", err)
	}
	return obj, true, nil
}

func (c *Client) GetList(ctx context.Context, path string) ([]map[string]any, error) {
	raw, _, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return list, nil
}

func (c *Client) writeObject(ctx context.Context, method, path string, body any) (map[string]any, error) {
	raw, _, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
	}
	return obj, nil
}

func (c *Client) Post(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.writeObject(ctx, http.MethodPost, path, body)
}

func (c *Client) Put(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.writeObject(ctx, http.MethodPut, path, body)
}

func (c *Client) Patch(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.writeObject(ctx, http.MethodPatch, path, body)
}

// A 404 is treated as success: the resource is already gone.
func (c *Client) Delete(ctx context.Context, path string) error {
	_, _, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}
