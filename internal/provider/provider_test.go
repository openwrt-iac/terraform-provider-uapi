package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// GetProviderSchema runs the framework's own schema validation across every
// resource and data source, catching Required+Computed conflicts and nested-type mistakes.
func TestProviderSchema(t *testing.T) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test")())()

	resp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema error: %v", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic: %s - %s", d.Summary, d.Detail)
		}
	}

	wantResources := []string{
		"uapi_firewall_rule", "uapi_firewall_zone", "uapi_firewall_redirect",
		"uapi_network_interface", "uapi_network_device",
		"uapi_wireless_device", "uapi_wireless_interface",
		"uapi_dhcp_host", "uapi_system",
		"uapi_authorized_key", "uapi_system_password",
		"uapi_mwan3_interface", "uapi_mwan3_globals", "uapi_usteer_config", "uapi_openvpn_instance",
	}
	for _, name := range wantResources {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource schema %q", name)
		}
	}

	wantDataSources := []string{
		"uapi_firewall_rule", "uapi_firewall_zone", "uapi_firewall_redirect",
		"uapi_network_interface", "uapi_network_device",
		"uapi_wireless_device", "uapi_wireless_interface",
		"uapi_dhcp_host", "uapi_system", "uapi_dhcp_leases",
		"uapi_dhcp_leases6", "uapi_authorized_key",
		"uapi_whoami", "uapi_healthz", "uapi_diagnostics",
	}
	for _, name := range wantDataSources {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("missing data source schema %q", name)
		}
	}

	if _, ok := resp.EphemeralResourceSchemas["uapi_token"]; !ok {
		t.Errorf("missing ephemeral resource schema %q", "uapi_token")
	}
}
