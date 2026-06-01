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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
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

// do performs a request with 423-retry. ifMatch, when set, is sent as the
// ?if_match= query parameter: uhttpd's CGI env drops the If-Match header, so
// uapi accepts the ETag via query string as the portable path. The response
// ETag (quoted) is returned so callers can persist it for later If-Match writes.
func (c *Client) do(ctx context.Context, method, path string, body any, ifMatch string) (respBody []byte, status int, etag string, err error) {
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, 0, "", fmt.Errorf("encoding request body: %w", err)
		}
	}

	target := c.baseURL + path
	if ifMatch != "" {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		target += sep + "if_match=" + url.QueryEscape(ifMatch)
	}

	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, reader)
		if err != nil {
			return nil, 0, "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		// Never log the body or token: requests carry secrets (keys, passwords).
		tflog.Debug(ctx, "uapi request", map[string]any{
			"method": method, "path": path, "if_match": ifMatch, "attempt": attempt,
		})
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, "", err
		}
		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, "", fmt.Errorf("reading response body: %w", readErr)
		}

		if resp.StatusCode == http.StatusLocked && attempt < maxLockRetries {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			select {
			case <-ctx.Done():
				return nil, resp.StatusCode, "", ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		etag = resp.Header.Get("ETag")
		tflog.Debug(ctx, "uapi response", map[string]any{
			"method": method, "path": path, "status": resp.StatusCode, "etag": etag,
		})
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return raw, resp.StatusCode, etag, nil
		}
		return nil, resp.StatusCode, etag, decodeError(resp.StatusCode, raw)
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

func statusIs(err error, code int) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == code
	}
	return false
}

// IsNotFound reports whether err is an API 404.
func IsNotFound(err error) bool { return statusIs(err, http.StatusNotFound) }

// IsPreconditionFailed reports whether err is an API 412 (stale If-Match).
func IsPreconditionFailed(err error) bool { return statusIs(err, http.StatusPreconditionFailed) }

// GetObject fetches a single resource and its ETag. found is false on 404.
func (c *Client) GetObject(ctx context.Context, path string) (obj map[string]any, etag string, found bool, err error) {
	raw, _, etag, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		if IsNotFound(err) {
			return nil, "", false, nil
		}
		return nil, "", false, err
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "", false, fmt.Errorf("decoding response: %w", err)
	}
	return obj, etag, true, nil
}

// GetList fetches a collection (read-only; no ETag tracking needed).
func (c *Client) GetList(ctx context.Context, path string) ([]map[string]any, error) {
	raw, _, _, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return list, nil
}

func (c *Client) writeObject(ctx context.Context, method, path string, body any, ifMatch string) (map[string]any, string, error) {
	raw, _, etag, err := c.do(ctx, method, path, body, ifMatch)
	if err != nil {
		return nil, "", err
	}
	var obj map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, "", fmt.Errorf("decoding response: %w", err)
		}
	}
	return obj, etag, nil
}

// Post creates a resource; ifMatch is normally empty for creation.
func (c *Client) Post(ctx context.Context, path string, body any, ifMatch string) (map[string]any, string, error) {
	return c.writeObject(ctx, http.MethodPost, path, body, ifMatch)
}

// Put replaces a resource, optionally guarded by If-Match.
func (c *Client) Put(ctx context.Context, path string, body any, ifMatch string) (map[string]any, string, error) {
	return c.writeObject(ctx, http.MethodPut, path, body, ifMatch)
}

// Patch partially updates a resource, optionally guarded by If-Match.
func (c *Client) Patch(ctx context.Context, path string, body any, ifMatch string) (map[string]any, string, error) {
	return c.writeObject(ctx, http.MethodPatch, path, body, ifMatch)
}

// Delete removes a resource, optionally guarded by If-Match. A 404 is success.
func (c *Client) Delete(ctx context.Context, path string, ifMatch string) error {
	_, _, _, err := c.do(ctx, http.MethodDelete, path, nil, ifMatch)
	if err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}
