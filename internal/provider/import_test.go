package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

func TestResolveImportIDManaged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r_managed","managed":true}`))
	}))
	defer srv.Close()

	id, adopted, err := resolveImportID(context.Background(), client.New(srv.URL, "t", false, "test"), "firewall/rules", "r_managed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adopted {
		t.Error("a managed section must not be adopted")
	}
	if id != "r_managed" {
		t.Errorf("id = %q", id)
	}
}

func TestResolveImportIDAdopts(t *testing.T) {
	var adoptCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"cfg0a1b","managed":false}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/adopt"):
			adoptCalled = true
			_, _ = w.Write([]byte(`{"id":"r_01HXNEW","managed":true}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	id, adopted, err := resolveImportID(context.Background(), client.New(srv.URL, "t", false, "test"), "firewall/rules", "cfg0a1b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !adoptCalled {
		t.Error("expected the adopt endpoint to be called")
	}
	if !adopted {
		t.Error("adopted should be true for an unmanaged section")
	}
	if id != "r_01HXNEW" {
		t.Errorf("id should be the new ULID from adopt, got %q", id)
	}
}
