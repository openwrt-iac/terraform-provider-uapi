package provider

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

func TestWriteErrPreconditionFailed(t *testing.T) {
	var d diag.Diagnostics
	writeErr(&d, "updating", "firewall zone", &client.APIError{Status: http.StatusPreconditionFailed, Code: "precondition_failed", Message: "stale"})
	if len(d.Errors()) != 1 {
		t.Fatalf("expected 1 error, got %d", len(d.Errors()))
	}
	e := d.Errors()[0]
	if !strings.Contains(e.Summary(), "changed outside Terraform") {
		t.Errorf("412 should surface a refresh hint, got summary %q", e.Summary())
	}
}

func TestWriteErrGeneric(t *testing.T) {
	var d diag.Diagnostics
	writeErr(&d, "creating", "dhcp host", &client.APIError{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: "bad"})
	if len(d.Errors()) != 1 || !strings.Contains(d.Errors()[0].Summary(), "creating dhcp host") {
		t.Errorf("generic write error wrong: %+v", d.Errors())
	}
}
