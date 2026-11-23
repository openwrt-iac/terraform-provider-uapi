package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

func invokeRecover(t *testing.T, srvURL, service string) (*action.InvokeResponse, []string) {
	t.Helper()
	ctx := context.Background()
	a := &serviceRecoverAction{client: client.New(srvURL, "t", false, "test")}
	schemaResp := &action.SchemaResponse{}
	a.Schema(ctx, action.SchemaRequest{}, schemaResp)
	cfg := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
			"service": tftypes.NewValue(tftypes.String, service),
		}),
	}
	var progress []string
	resp := &action.InvokeResponse{SendProgress: func(e action.InvokeProgressEvent) { progress = append(progress, e.Message) }}
	a.Invoke(ctx, action.InvokeRequest{Config: cfg}, resp)
	return resp, progress
}

func TestServiceRecoverAction(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"dnsmasq","recovered":["cfg01411c"]}`))
	}))
	defer srv.Close()

	resp, progress := invokeRecover(t, srv.URL, "dnsmasq")
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if gotMethod != http.MethodPost || gotPath != "/uapi/services/dnsmasq/recover" {
		t.Errorf("request = %s %s, want POST /uapi/services/dnsmasq/recover", gotMethod, gotPath)
	}
	if len(progress) != 1 || !strings.Contains(progress[0], "cfg01411c") {
		t.Errorf("progress = %v, want the recovered instance named", progress)
	}
}

func TestServiceRecoverActionNothingToRecover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"service":"snmpd","recovered":[]}`))
	}))
	defer srv.Close()

	resp, progress := invokeRecover(t, srv.URL, "snmpd")
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if len(progress) != 1 || !strings.Contains(progress[0], "started nothing") {
		t.Errorf("progress = %v, want that uapi started nothing", progress)
	}
}

// A 500 recover_failed is an error: an instance is still down.
func TestServiceRecoverActionFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"recover_failed","message":"dnsmasq/cfg01411c is still down","service":"dnsmasq","stopped":["cfg01411c"]}`))
	}))
	defer srv.Close()

	resp, _ := invokeRecover(t, srv.URL, "dnsmasq")
	if !resp.Diagnostics.HasError() {
		t.Fatal("want an error for 500 recover_failed")
	}
	if d := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(d, "recover_failed") || !strings.Contains(d, "still down") {
		t.Errorf("error detail = %q, want the code and the message", d)
	}
}
