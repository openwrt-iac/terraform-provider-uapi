package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func testClient(url string) *Client {
	c := New(url, "tok", false)
	return c
}

func TestGetObjectFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("missing bearer header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"r_1","managed":true,"enabled":true}`))
	}))
	defer srv.Close()

	obj, found, err := testClient(srv.URL).GetObject(context.Background(), "/firewall/rules/r_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if obj["id"] != "r_1" {
		t.Errorf("id = %v", obj["id"])
	}
	if obj["managed"] != true {
		t.Errorf("managed = %v", obj["managed"])
	}
}

func TestGetObjectNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":"not_found","message":"nope"}`))
	}))
	defer srv.Close()

	_, found, err := testClient(srv.URL).GetObject(context.Background(), "/x/y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestValidationErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"code":"validation_failed","message":"bad","errors":[{"field":"target","code":"required","message":"is required"}]}`))
	}))
	defer srv.Close()

	_, err := testClient(srv.URL).Post(context.Background(), "/firewall/rules", map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "validation_failed" {
		t.Errorf("code = %q", apiErr.Code)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Field != "target" {
		t.Errorf("field errors not parsed: %+v", apiErr.Errors)
	}
}

func TestRetryOnLocked(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusLocked)
			w.Write([]byte(`{"code":"locked","message":"busy"}`))
			return
		}
		w.Write([]byte(`{"id":"r_1"}`))
	}))
	defer srv.Close()

	obj, err := testClient(srv.URL).Put(context.Background(), "/firewall/rules/r_1", map[string]any{"target": "ACCEPT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj["id"] != "r_1" {
		t.Errorf("id = %v", obj["id"])
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 calls (2 locked + 1 ok), got %d", got)
	}
}

func TestLockedExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusLocked)
		w.Write([]byte(`{"code":"locked","message":"busy"}`))
	}))
	defer srv.Close()

	_, err := testClient(srv.URL).Post(context.Background(), "/firewall/rules", map[string]any{})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if apiErr, ok := err.(*APIError); !ok || apiErr.Status != http.StatusLocked {
		t.Errorf("expected locked APIError, got %v", err)
	}
}

func TestDeleteToleratesNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":"not_found"}`))
	}))
	defer srv.Close()

	if err := testClient(srv.URL).Delete(context.Background(), "/firewall/rules/gone"); err != nil {
		t.Fatalf("delete should tolerate 404, got %v", err)
	}
}

func TestGetList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"mac":"aa:bb:cc:dd:ee:ff","ip":"10.0.0.2"}]`))
	}))
	defer srv.Close()

	list, err := testClient(srv.URL).GetList(context.Background(), "/dhcp/leases")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 || list[0]["ip"] != "10.0.0.2" {
		t.Errorf("unexpected list: %+v", list)
	}
}
