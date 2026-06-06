// Package client is a thin HTTP client for the uapi REST API.
package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// lockRetryBudget bounds, by wall-clock, how long a single request retries the
// throttling statuses 423 (locked) and 429 (rate limited). Time-bounded rather
// than attempt-bounded so the default Terraform parallelism (10 concurrent
// same-package writers all contending one lock) drains through instead of
// exhausting a small attempt count. maxThrottleAttempts is a runaway backstop.
const (
	lockRetryBudget     = 45 * time.Second
	maxThrottleAttempts = 50
)

// maxPages bounds cursor-pagination so a misbehaving server cannot loop forever.
const maxPages = 1000

func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

type Client struct {
	baseURL   string
	token     string
	userAgent string
	http      *http.Client
}

// insecure skips TLS verification, needed against uapi's default self-signed
// certificate. version is the provider version, surfaced in the User-Agent.
func New(baseURL, token string, insecure bool, version string) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if version == "" {
		version = "dev"
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		userAgent: fmt.Sprintf("terraform-provider-uapi/%s (%s/%s)", version, runtime.GOOS, runtime.GOARCH),
		http:      &http.Client{Timeout: 60 * time.Second, Transport: transport},
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

// do performs a request, retrying the throttling statuses 423/429 (honoring
// Retry-After). ifMatch, when set, is sent as the ?if_match= query parameter
// (uhttpd's CGI env drops the header). POSTs carry a stable Idempotency-Key so a
// retried create cannot double-apply. Returns the response ETag and, for
// collection GETs, the X-Next-Cursor for pagination.
func (c *Client) do(ctx context.Context, method, path string, body any, ifMatch string) (respBody []byte, status int, etag, nextCursor string, err error) {
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, 0, "", "", fmt.Errorf("encoding request body: %w", err)
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

	// One idempotency key per logical create, reused across this call's retries.
	idemKey := ""
	if method == http.MethodPost {
		idemKey = newIdempotencyKey()
	}

	var throttleDeadline time.Time // set on the first 423/429; bounds retry wall-clock
	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, reader)
		if err != nil {
			return nil, 0, "", "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
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
			return nil, 0, "", "", err
		}
		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, "", "", fmt.Errorf("reading response body: %w", readErr)
		}

		throttled := resp.StatusCode == http.StatusLocked || resp.StatusCode == http.StatusTooManyRequests
		if throttled {
			if throttleDeadline.IsZero() {
				throttleDeadline = time.Now().Add(lockRetryBudget)
			}
			if attempt < maxThrottleAttempts && time.Now().Before(throttleDeadline) {
				wait := retryWait(resp.Header.Get("Retry-After"), attempt)
				select {
				case <-ctx.Done():
					return nil, resp.StatusCode, "", "", ctx.Err()
				case <-time.After(wait):
				}
				continue
			}
		}

		etag = resp.Header.Get("ETag")
		nextCursor = resp.Header.Get("X-Next-Cursor")
		// X-Reload-Status is uapi's "did the init-script reload run" signal (ok |
		// no_reload). 2xx means the write committed, not that the daemon converged;
		// surface it at debug for "applied but nothing changed" diagnosis.
		tflog.Debug(ctx, "uapi response", map[string]any{
			"method": method, "path": path, "status": resp.StatusCode, "etag": etag,
			"reload_status":   resp.Header.Get("X-Reload-Status"),
			"reload_services": resp.Header.Get("X-Reload-Services"),
		})
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return raw, resp.StatusCode, etag, nextCursor, nil
		}
		return nil, resp.StatusCode, etag, nextCursor, decodeError(resp.StatusCode, raw)
	}
}

// retryWait honors a server Retry-After when present; otherwise it falls back to
// exponential backoff with jitter, so concurrent clients retrying a 423/429 do
// not synchronize into a thundering herd.
func retryWait(header string, attempt int) time.Duration {
	if header != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	const (
		base    = 200 * time.Millisecond
		maxWait = 5 * time.Second
	)
	// Cap the shift exponent: 200ms<<5 already exceeds maxWait, and a large
	// attempt would overflow int64 to a negative the cap below would not catch.
	backoff := maxWait
	if attempt < 5 {
		backoff = base << attempt // 200ms, 400ms, 800ms, 1.6s, 3.2s
	}
	return backoff + time.Duration(mrand.Int63n(int64(base)))
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
	raw, _, etag, _, err := c.do(ctx, http.MethodGet, path, nil, "")
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

// GetList fetches a collection, following cursor pagination (X-Next-Cursor)
// until the server stops returning a next cursor.
func (c *Client) GetList(ctx context.Context, path string) ([]map[string]any, error) {
	var all []map[string]any
	next := ""
	for page := 0; page < maxPages; page++ {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		p := path + sep + "limit=500"
		if next != "" {
			p += "&cursor=" + url.QueryEscape(next)
		}
		raw, _, _, cursor, err := c.do(ctx, http.MethodGet, p, nil, "")
		if err != nil {
			return nil, err
		}
		var list []map[string]any
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		all = append(all, list...)
		if cursor == "" {
			return all, nil
		}
		next = cursor
	}
	// Hit the page bound with the server still offering a next cursor: return what
	// we have but make the truncation visible rather than silently capping.
	tflog.Warn(ctx, "uapi list truncated at page bound", map[string]any{
		"path": path, "max_pages": maxPages, "items": len(all),
	})
	return all, nil
}

func (c *Client) writeObject(ctx context.Context, method, path string, body any, ifMatch string) (map[string]any, string, error) {
	raw, _, etag, _, err := c.do(ctx, method, path, body, ifMatch)
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
	_, _, _, _, err := c.do(ctx, http.MethodDelete, path, nil, ifMatch)
	if err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}
