package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// accProvidersWithEcho adds the test "echo" provider, which mirrors an ephemeral
// resource's value into ordinary state so it can be asserted.
func accProvidersWithEcho() map[string]func() (tfprotov6.ProviderServer, error) {
	p := accProviders()
	p["echo"] = echoprovider.NewProviderServer()
	return p
}

// accTarget returns the provider configuration block a test should use and a
// closer to release any resources it allocated. When UAPI_ACC_ENDPOINT is set
// the suite runs against that live router (UAPI_ACC_TOKEN supplies the token);
// otherwise it spins up a fresh in-process mock. Only endpoint-agnostic tests
// (CRUD + id/managed/etag shape) should use this; mock-value-specific tests
// instantiate the mock directly.
func accTarget(t *testing.T) (providerCfg string, closer func()) {
	t.Helper()
	if ep := os.Getenv("UAPI_ACC_ENDPOINT"); ep != "" {
		cfg := fmt.Sprintf("provider \"uapi\" {\n  endpoint = %q\n  token    = %q\n  insecure = true\n}\n",
			ep, os.Getenv("UAPI_ACC_TOKEN"))
		return cfg, func() {}
	}
	m := newMockUAPI()
	return providerHCL(m.URL), m.Close
}

// accProviders serves the real provider in-process; tests point its endpoint at
// a per-test mock uapi server. resource.Test auto-skips unless TF_ACC=1.
func accProviders() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"uapi": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func providerHCL(url string) string {
	return fmt.Sprintf("provider \"uapi\" {\n  endpoint = %q\n  token    = \"test\"\n}\n", url)
}

func TestAccFirewallZone(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_firewall_zone" "z" {
  name = "dmz"
  masq = true
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("uapi_firewall_zone.z", "id"),
					resource.TestCheckResourceAttrSet("uapi_firewall_zone.z", "etag"),
					resource.TestCheckResourceAttr("uapi_firewall_zone.z", "managed", "true"),
					resource.TestCheckResourceAttr("uapi_firewall_zone.z", "name", "dmz"),
					resource.TestCheckResourceAttr("uapi_firewall_zone.z", "masq", "true"),
				),
			},
			{
				// update in place
				Config: providerHCL(m.URL) + `
resource "uapi_firewall_zone" "z" {
  name = "dmz"
  masq = false
}`,
				Check: resource.TestCheckResourceAttr("uapi_firewall_zone.z", "masq", "false"),
			},
			{
				ResourceName:      "uapi_firewall_zone.z",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFirewallRule_nestedMatch(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_firewall_rule" "r" {
  target = "ACCEPT"
  match = {
    src_zone  = "wan"
    proto     = ["tcp"]
    dest_port = ["22"]
  }
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("uapi_firewall_rule.r", "target", "ACCEPT"),
				resource.TestCheckResourceAttr("uapi_firewall_rule.r", "match.src_zone", "wan"),
				resource.TestCheckResourceAttr("uapi_firewall_rule.r", "match.proto.0", "tcp"),
				resource.TestCheckResourceAttr("uapi_firewall_rule.r", "match.dest_port.0", "22"),
			),
		}},
	})
}

func TestAccWirelessInterface_writeOnlyKey(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_wireless_interface" "w" {
  device     = "radio0"
  ssid       = "home"
  encryption = "psk2"
  key        = "supersecret"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_wireless_interface.w", "ssid", "home"),
					resource.TestCheckResourceAttr("uapi_wireless_interface.w", "key", "supersecret"),
					resource.TestCheckResourceAttr("uapi_wireless_interface.w", "has_key", "true"),
				),
			},
			{
				ResourceName:            "uapi_wireless_interface.w",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"}, // write-only, never returned
			},
		},
	})
}

func TestAccSystem_singleton(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_system" "this" {
  hostname = "edge"
}`,
				Check: resource.TestCheckResourceAttr("uapi_system.this", "hostname", "edge"),
			},
			{
				Config: providerHCL(m.URL) + `
resource "uapi_system" "this" {
  hostname = "edge2"
}`,
				Check: resource.TestCheckResourceAttr("uapi_system.this", "hostname", "edge2"),
			},
		},
	})
}

func TestAccFirewallZone_importAdopt(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	// Pre-existing anonymous section; import should adopt it (new managed id).
	m.seedUnmanaged("/firewall/zones", "cfg0a1b", map[string]any{"name": "legacy", "input": "DROP"})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_firewall_zone" "legacy" {
  name = "legacy"
}`,
			ResourceName:       "uapi_firewall_zone.legacy",
			ImportState:        true,
			ImportStateId:      "cfg0a1b",
			ImportStatePersist: true,
			ImportStateCheck: func(states []*terraform.InstanceState) error {
				if len(states) != 1 {
					return fmt.Errorf("expected 1 state, got %d", len(states))
				}
				s := states[0]
				if s.Attributes["managed"] != "true" {
					return fmt.Errorf("adopted section should be managed, got %q", s.Attributes["managed"])
				}
				if s.ID == "cfg0a1b" {
					return fmt.Errorf("adopt should have assigned a new id, still %q", s.ID)
				}
				return nil
			},
		}},
	})
}

func TestAccAuthorizedKey(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_authorized_key" "k" {
  key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAImock me@host"
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("uapi_authorized_key.k", "type", "ssh-ed25519"),
				resource.TestCheckResourceAttr("uapi_authorized_key.k", "comment", "me@host"),
				resource.TestCheckResourceAttrSet("uapi_authorized_key.k", "id"),
			),
		}},
	})
}

func TestAccSystemPassword_writeOnly(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_system_password" "root" {
  user                = "root"
  password_wo         = "hunter2hunter2"
  password_wo_version = "1"
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("uapi_system_password.root", "user", "root"),
				resource.TestCheckResourceAttr("uapi_system_password.root", "id", "root"),
				resource.TestCheckNoResourceAttr("uapi_system_password.root", "password_wo"), // write-only: never in state
			),
		}},
	})
}

func TestAccPackagePreExisted(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	// Seed the package as already installed; managing it must record pre_existed=true
	// so destroy will skip the uninstall.
	m.seedUnmanaged("/packages/installed", "curl", map[string]any{"name": "curl", "installed": true})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_package" "c" {
  name = "curl"
}`,
			Check: resource.TestCheckResourceAttr("uapi_package.c", "pre_existed", "true"),
		}},
	})
}

func TestAccDataSources(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_network_interface" "lan" {
  proto  = "static"
  ipaddr = "192.168.1.1"
}

data "uapi_network_interface" "lan" {
  id = uapi_network_interface.lan.id
}

data "uapi_dhcp_leases" "all" {}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				// runtime block surfaced on the data source
				resource.TestCheckResourceAttr("data.uapi_network_interface.lan", "runtime.up", "true"),
				resource.TestCheckResourceAttr("data.uapi_network_interface.lan", "runtime.l3_device", "br-lan"),
				resource.TestCheckResourceAttr("data.uapi_network_interface.lan", "runtime.ipv4_address.0.address", "192.168.1.1"),
				// list data source
				resource.TestCheckResourceAttr("data.uapi_dhcp_leases.all", "leases.0.ip", "192.168.1.50"),
			),
		}},
	})
}

func TestAccOpsDataSources(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
data "uapi_whoami" "me" {}
data "uapi_healthz" "h" {}
data "uapi_diagnostics" "d" {}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.uapi_whoami.me", "token_id", "acc"),
				resource.TestCheckResourceAttr("data.uapi_whoami.me", "scopes.0", "network:write"),
				resource.TestCheckResourceAttr("data.uapi_healthz.h", "status", "ok"),
				resource.TestCheckResourceAttr("data.uapi_healthz.h", "checks.ubus", "ok"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.d", "uptime_seconds", "12345"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.d", "lock_state.global_held", "false"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.d", "recent_errors.0.code", "validation_failed"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.d", "recent_errors.0.status", "422"),
			),
		}},
	})
}

func TestAccTokenEphemeral(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV6ProviderFactories: accProvidersWithEcho(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
ephemeral "uapi_token" "t" {
  name   = "ci"
  scopes = ["network:write"]
}

provider "echo" {
  data = ephemeral.uapi_token.t.token
}

resource "echo" "minted" {}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("echo.minted", "data", "deadbeefdeadbeefdeadbeefdeadbeef"),
			),
		}},
	})
}
