package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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

// accErrorCheck skips a live case that the router cannot host: uapi (>= 3.2.0)
// answers 503 to a write into a package whose config file or init script is
// missing there, such as /etc/config/wireless on a box without a radio. The mock
// never answers these, so a mock run still fails on them.
func accErrorCheck(t *testing.T) resource.ErrorCheckFunc {
	return func(err error) error {
		if reason := unhostableReason(err); reason != "" && os.Getenv("UAPI_ACC_ENDPOINT") != "" {
			t.Skip(reason)
		}
		return err
	}
}

var (
	unhostable = regexp.MustCompile(`uapi 503 (config_missing|init_script_missing): `)
	blankLine  = regexp.MustCompile(`\n\s*\n`)
)

// unhostableReason returns the uapi message of such a 503, unwrapped from
// Terraform's diagnostic output, or "" when err is not one.
func unhostableReason(err error) string {
	for _, p := range blankLine.Split(err.Error(), -1) {
		p = strings.Join(strings.Fields(p), " ")
		if loc := unhostable.FindStringIndex(p); loc != nil {
			return p[loc[0]:]
		}
	}
	return ""
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

// firewall/nat's match block is the third match shape: nested like a rule, but
// with scalar addresses and ports rather than lists, because firewall4 parses a
// `config nat` section's options as scalars. Only `proto` stays a list.
func TestAccFirewallNat_scalarMatch(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_firewall_nat" "n" {
  target = "SNAT"
  snat_ip = "203.0.113.7"
  match = {
    src_zone = "wan"
    src_ip   = "192.168.9.0/24"
    proto    = ["tcp"]
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("uapi_firewall_nat.n", "id"),
					resource.TestCheckResourceAttrSet("uapi_firewall_nat.n", "etag"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "managed", "true"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "target", "SNAT"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "snat_ip", "203.0.113.7"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "match.src_zone", "wan"),
					// scalar, not match.src_ip.0
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "match.src_ip", "192.168.9.0/24"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "match.proto.0", "tcp"),
				),
			},
			{
				// update in place, including a scalar match field
				Config: providerHCL(m.URL) + `
resource "uapi_firewall_nat" "n" {
  target = "MASQUERADE"
  match = {
    src_zone  = "wan"
    src_ip    = "192.168.10.0/24"
    dest_port = "443"
    proto     = ["tcp"]
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "target", "MASQUERADE"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "match.src_ip", "192.168.10.0/24"),
					resource.TestCheckResourceAttr("uapi_firewall_nat.n", "match.dest_port", "443"),
				),
			},
			{
				ResourceName:      "uapi_firewall_nat.n",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// A nat section that matches all egress carries an empty match block rather than
// no match block: the schema requires it (see the note in renderers.go).
func TestAccFirewallNat_emptyMatch(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_firewall_nat" "all" {
  target = "MASQUERADE"
  match  = {}
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("uapi_firewall_nat.all", "id"),
				resource.TestCheckResourceAttr("uapi_firewall_nat.all", "target", "MASQUERADE"),
			),
		}},
	})
}

// The validation sweep is opt-in because it re-validates every section on the
// router. With validate set, invalid_sections reports what a write would reject
// today, and skipped_for_scope says where the token was not allowed to look, so
// an empty result is not mistaken for a clean router.
func TestAccDiagnostics_validationSweep(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
data "uapi_diagnostics" "swept" {
  validate = true
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "invalid_sections.#", "1"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "invalid_sections.0.resource", "firewall/rules"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "invalid_sections.0.id", "cfg0a1b2c"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "invalid_sections.0.managed", "false"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "invalid_sections.0.errors.0.field", "match.dest_port"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "swept_resources.0", "firewall:rules"),
				resource.TestCheckResourceAttr("data.uapi_diagnostics.swept", "skipped_for_scope.0", "dhcp:hosts"),
			),
		}},
	})
}

// A router with nothing wrong sends an empty invalid_sections. The decode must
// still produce an empty list rather than leaving the slice nil, because the
// framework reflects a nil slice to a null attribute and length()/for_each would
// then fail on the result that means the router is clean.
func TestAccDiagnostics_emptySweepIsEmptyNotNull(t *testing.T) {
	m := newMockUAPI()
	m.emptySweep = true
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
data "uapi_diagnostics" "clean" {
  validate = true
}
output "invalid_count" {
  value = length(data.uapi_diagnostics.clean.invalid_sections)
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.uapi_diagnostics.clean", "invalid_sections.#", "0"),
				resource.TestCheckOutput("invalid_count", "0"),
			),
		}},
	})
}

// dhcp/hosts.tag is a list as of uapi 2.5.0: a response is always an array, even
// for a section stored as a space-separated scalar. match_tag (uapi 3.1.0) is a
// list too, and dnsmasq reads only the list form, so a single tag stays a list.
func TestAccDhcpHost_tagList(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerCfg + `
resource "uapi_dhcp_host" "h" {
  name      = "uapiacc-printer"
  macs      = ["02:00:00:00:31:01"]
  tag       = ["red", "blue"]
  match_tag = ["vc"]
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("uapi_dhcp_host.h", "tag.0", "red"),
				resource.TestCheckResourceAttr("uapi_dhcp_host.h", "tag.1", "blue"),
				resource.TestCheckResourceAttr("uapi_dhcp_host.h", "match_tag.#", "1"),
				resource.TestCheckResourceAttr("uapi_dhcp_host.h", "match_tag.0", "vc"),
				resource.TestCheckResourceAttr("uapi_dhcp_host.h", "macs.0", "02:00:00:00:31:01"),
			),
		}},
	})
}

// uci cannot store an empty list, so uapi drops `[]` and reads null back. A
// configured `[]` must still apply cleanly, which is the only way to clear an
// Optional+Computed list, flat or inside `match`.
func TestAccEmptyListClears(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	cfg := func(tags, srcIP string) string {
		return providerCfg + fmt.Sprintf(`
resource "uapi_dhcp_host" "h" {
  macs      = ["02:00:00:00:31:02"]
  tag       = %[1]s
  match_tag = %[1]s
}

resource "uapi_firewall_rule" "r" {
  target = "ACCEPT"
  match  = {
    src_zone = "wan"
    src_ip   = %[2]s
  }
}`, tags, srcIP)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: cfg(`["vc"]`, `["192.0.2.1"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_dhcp_host.h", "match_tag.#", "1"),
					resource.TestCheckResourceAttr("uapi_firewall_rule.r", "match.src_ip.#", "1"),
				),
			},
			{
				Config: cfg(`[]`, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_dhcp_host.h", "tag.#", "0"),
					resource.TestCheckResourceAttr("uapi_dhcp_host.h", "match_tag.#", "0"),
					resource.TestCheckResourceAttr("uapi_firewall_rule.r", "match.src_ip.#", "0"),
				),
			},
		},
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

func TestAccUnboundSrv_singletonLists(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_unbound_srv" "this" {
  interface_bind = ["127.0.0.1@5353"]
  srv_line       = ["do-ip6: no"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_unbound_srv.this", "interface_bind.0", "127.0.0.1@5353"),
					resource.TestCheckResourceAttr("uapi_unbound_srv.this", "srv_line.0", "do-ip6: no"),
				),
			},
			{
				Config: providerHCL(m.URL) + `
resource "uapi_unbound_srv" "this" {
  interface_bind = ["127.0.0.1@5353", "::1@5353"]
  srv_line       = ["do-ip6: no"]
}`,
				Check: resource.TestCheckResourceAttr("uapi_unbound_srv.this", "interface_bind.1", "::1@5353"),
			},
		},
	})
}

// TestAccSettableID covers the 2.2.0 settable id: a chosen id becomes the
// section name, and changing it forces replacement (create-only).
func TestAccSettableID(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_dhcp_host" "h" {
  id = "myhost"
  ip = "192.168.9.5"
}`,
				Check: resource.TestCheckResourceAttr("uapi_dhcp_host.h", "id", "myhost"),
			},
			{
				Config: providerHCL(m.URL) + `
resource "uapi_dhcp_host" "h" {
  id = "myhost2"
  ip = "192.168.9.5"
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("uapi_dhcp_host.h", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr("uapi_dhcp_host.h", "id", "myhost2"),
			},
		},
	})
}

// TestAccClearOnOmit covers the 2.2.3 spec-driven clear-on-omit: netmask/gateway
// are plain Optional, so removing them from config clears them (an in-place update
// that converges), instead of Optional+Computed stickiness hiding the omission.
func TestAccClearOnOmit(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_network_interface" "clr" {
  id      = "clearif"
  proto   = "static"
  ipaddrs = ["192.168.50.1"]
  netmask = "255.255.255.0"
  gateway = "192.168.50.254"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_network_interface.clr", "netmask", "255.255.255.0"),
					resource.TestCheckResourceAttr("uapi_network_interface.clr", "gateway", "192.168.50.254"),
				),
			},
			{
				// Drop netmask + gateway: in-place update that clears them, NOT sticky.
				Config: providerHCL(m.URL) + `
resource "uapi_network_interface" "clr" {
  id     = "clearif"
  proto  = "static"
  ipaddrs = ["192.168.50.1"]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("uapi_network_interface.clr", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("uapi_network_interface.clr", "netmask"),
					resource.TestCheckNoResourceAttr("uapi_network_interface.clr", "gateway"),
					// ipaddr is not clear-on-omit (not flagged): it must survive.
					resource.TestCheckResourceAttr("uapi_network_interface.clr", "ipaddr", "192.168.50.1"),
				),
			},
		},
	})
}

// Renumbering an interface must not fail the apply. `ipaddr` is derived by uapi
// from the first `ipaddrs` entry, so a plan that carries the prior `ipaddr`
// forward while `ipaddrs` changes ends in "Provider produced inconsistent result
// after apply" (issue #21, reported against 2.4.0). 3.0.0 made `ipaddr`
// response-only, emitted as a bare Computed attribute with no
// UseStateForUnknown, so it plans as unknown and any post-apply value satisfies
// it. Retiring the mirrored-pair machinery deleted the three tests that covered
// this, so the scenario went uncovered even though the behaviour is what
// changed: this asserts the outcome the issue reported, not the mechanism.
func TestAccNetworkInterface_renumberDoesNotFailApply(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_network_interface" "renum" {
  id      = "renumif"
  proto   = "static"
  ipaddrs = ["192.0.2.4"]
  netmask = "255.255.255.0"
}`,
				Check: resource.TestCheckResourceAttr("uapi_network_interface.renum", "ipaddr", "192.0.2.4"),
			},
			{
				Config: providerHCL(m.URL) + `
resource "uapi_network_interface" "renum" {
  id      = "renumif"
  proto   = "static"
  ipaddrs = ["192.0.2.1"]
  netmask = "255.255.255.0"
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("uapi_network_interface.renum", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_network_interface.renum", "ipaddrs.0", "192.0.2.1"),
					// The derived view has to follow the list, not the prior state.
					resource.TestCheckResourceAttr("uapi_network_interface.renum", "ipaddr", "192.0.2.1"),
				),
			},
		},
	})
}

const wgTestKey = "4TMrf9VJAdq0+HDQRFG9uuRYXTtMmu6PrCwpaomeICk="

// A proto switch must apply with the attributes of the old proto removed from
// the configuration. They are Optional+Computed, so UseStateForUnknown kept
// their prior values, the PUT sent them, and uapi >= 3.2.0 refused it with 422
// (#32). The plan after each step must be empty.
func TestAccNetworkInterface_protoSwitch(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	const n = "uapi_network_interface.sw"
	iface := func(body string) string {
		return providerCfg + "resource \"uapi_network_interface\" \"sw\" {\n  id = \"uapiacc_sw\"\n" + body + "}\n"
	}
	inPlace := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(n, plancheck.ResourceActionUpdate),
	}}
	static := `  proto   = "static"
  ipaddrs = ["192.0.2.10"]
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: iface(`  proto    = "static"
  ipaddrs  = ["192.0.2.10"]
  ip6addrs = ["2001:db8:32::10/64"]
  netmask  = "255.255.255.0"
`),
				Check: resource.TestCheckResourceAttr(n, "ip6addrs.0", "2001:db8:32::10/64"),
			},
			{
				Config: iface(`  proto    = "dhcp"
  hostname = "uapiacc-sw"
  clientid = "01020000003201"
  peerdns  = false
`),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(n, "hostname", "uapiacc-sw"),
					resource.TestCheckNoResourceAttr(n, "ipaddrs.#"),
					resource.TestCheckNoResourceAttr(n, "ip6addrs.#"),
					resource.TestCheckNoResourceAttr(n, "netmask"),
				),
			},
			{
				Config:           iface(static),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(n, "ipaddrs.0", "192.0.2.10"),
					resource.TestCheckNoResourceAttr(n, "hostname"),
					resource.TestCheckNoResourceAttr(n, "clientid"),
					resource.TestCheckNoResourceAttr(n, "peerdns"),
				),
			},
			{
				Config: iface(fmt.Sprintf(`  proto       = "wireguard"
  private_key = %q
  listen_port = 51998
  addresses   = ["10.32.0.1/24"]
`, wgTestKey)),
				ConfigPlanChecks: inPlace,
				Check:            resource.TestCheckResourceAttr(n, "listen_port", "51998"),
			},
			{
				Config:           iface(static),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(n, "addresses.#"),
					resource.TestCheckNoResourceAttr(n, "listen_port"),
					resource.TestCheckNoResourceAttr(n, "nohostroute"),
				),
			},
		},
	})
}

// The options that netifd reads for every proto must apply on each proto and
// survive a switch. Before uapi 3.2.0 some protos dropped them behind a 200, and
// uapi stored ip6hint only on dhcpv6, so a static interface failed the apply
// with an inconsistent result (#31).
func TestAccNetworkInterface_coreOptions(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	const n = "uapi_network_interface.core"
	iface := func(body string) string {
		return providerCfg + `resource "uapi_network_interface" "core" {
  id           = "uapiacc_core"
  metric       = 20
  ip4table     = "100"
  ip6table     = "100"
  defaultroute = false
  delegate     = false
  ip6assign    = 64
  ip6hint      = "1"
  ip6class     = ["local"]
  ip6weight    = 10
` + body + "}\n"
	}
	core := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(n, "metric", "20"),
		resource.TestCheckResourceAttr(n, "ip4table", "100"),
		resource.TestCheckResourceAttr(n, "ip6table", "100"),
		resource.TestCheckResourceAttr(n, "defaultroute", "false"),
		resource.TestCheckResourceAttr(n, "delegate", "false"),
		resource.TestCheckResourceAttr(n, "ip6assign", "64"),
		resource.TestCheckResourceAttr(n, "ip6hint", "1"),
		resource.TestCheckResourceAttr(n, "ip6class.0", "local"),
		resource.TestCheckResourceAttr(n, "ip6weight", "10"),
	)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: iface(`  proto   = "static"
  ipaddrs = ["192.0.2.11"]
`),
				Check: core,
			},
			{
				Config: iface(`  proto        = "dhcpv6"
  sourcefilter = false
  reqprefix    = "2001:db8::/56"
  reqaddress   = "try"
  clientid     = "00030001020000003202"
`),
				Check: resource.ComposeAggregateTestCheckFunc(core,
					resource.TestCheckResourceAttr(n, "sourcefilter", "false"),
					resource.TestCheckResourceAttr(n, "reqprefix", "2001:db8::/56"),
					resource.TestCheckNoResourceAttr(n, "ipaddrs.#"),
				),
			},
			{
				Config: iface(`  proto          = "dhcp"
  broadcast_flag = true
`),
				Check: resource.ComposeAggregateTestCheckFunc(core,
					resource.TestCheckResourceAttr(n, "broadcast_flag", "true"),
					resource.TestCheckNoResourceAttr(n, "broadcast"),
					resource.TestCheckNoResourceAttr(n, "sourcefilter"),
					resource.TestCheckNoResourceAttr(n, "reqprefix"),
				),
			},
		},
	})
}

// A value for an attribute that the proto does not take fails the plan, before
// uapi can refuse the apply.
func TestAccNetworkInterface_scopeRefusedAtPlan(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerCfg + `
resource "uapi_network_interface" "bad" {
  id       = "uapiacc_bad"
  proto    = "static"
  ipaddrs  = ["192.0.2.12"]
  hostname = "uapiacc-bad"
  dns      = []
}`,
			ExpectError: regexp.MustCompile(`proto "static" does not take hostname`),
		}},
	})
}

// When the proto is unknown at plan time, no prior value is safe to send: the
// scoped attributes plan as unknown, so the PUT leaves out the static address
// that a switch to dhcp would have carried.
func TestAccNetworkInterface_unknownProto(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	cfg := func(proto, addrs string) string {
		return providerCfg + fmt.Sprintf(`
resource "terraform_data" "proto" {
  input = %q
}

resource "uapi_network_interface" "u" {
  id    = "uapiacc_unk"
  proto = terraform_data.proto.output
%s}
`, proto, addrs)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: cfg("static", "  ipaddrs = [\"192.0.2.13\"]\n"),
				Check:  resource.TestCheckResourceAttr("uapi_network_interface.u", "ipaddrs.0", "192.0.2.13"),
			},
			{
				Config: cfg("dhcp", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("uapi_network_interface.u", "proto", "dhcp"),
					resource.TestCheckNoResourceAttr("uapi_network_interface.u", "ipaddrs.#"),
				),
			},
		},
	})
}

// ip6prefix is a list as of uapi 3.2.0 (#34). uapi stores one prefix as a uci
// option and several as a uci list, and reads both as an array, so the import
// step reads back the scalar that the second step stored.
func TestAccNetworkInterface_ip6prefixList(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	const n = "uapi_network_interface.p6"
	iface := func(prefix string) string {
		return providerCfg + fmt.Sprintf(`resource "uapi_network_interface" "p6" {
  id       = "uapiacc_p6"
  proto    = "static"
  ip6addrs = ["2001:db8:34::1/64"]
%s}
`, prefix)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: iface(`  ip6prefix = ["2001:db8:340::/48", "2001:db8:341::/48"]` + "\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(n, "ip6prefix.#", "2"),
					resource.TestCheckResourceAttr(n, "ip6prefix.0", "2001:db8:340::/48"),
					resource.TestCheckResourceAttr(n, "ip6prefix.1", "2001:db8:341::/48"),
				),
			},
			{
				Config: iface(`  ip6prefix = ["2001:db8:340::/48"]` + "\n"),
				Check:  resource.TestCheckResourceAttr(n, "ip6prefix.#", "1"),
			},
			{
				ResourceName:      n,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: iface(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(n, plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckNoResourceAttr(n, "ip6prefix.#"),
			},
		},
	})
}

// An interface on a proto that uapi does not model, such as a 6in4 tunnel that
// LuCI created, imports and updates (#35). uapi refuses a PUT there, and a PUT
// would drop the options of the proto's handler, so an update is a merge PATCH
// of the changes. A switch to a modelled proto is a PUT, which replaces them.
func TestAccNetworkInterface_unmodelledProto(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	m.seedUnmanaged("/network/interfaces", "t6in4", map[string]any{
		"proto": "6in4", "peeraddr": "192.0.2.1", "ip6prefix": []any{"2001:db8:46::/48"},
	})
	const n = "uapi_network_interface.tun"
	tun := func(body string) string {
		return providerHCL(m.URL) + "resource \"uapi_network_interface\" \"tun\" {\n  id = \"t6in4\"\n" + body + "}\n"
	}
	stored := func(key string, want any) resource.TestCheckFunc {
		return func(*terraform.State) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			got, ok := m.store["/network/interfaces"]["t6in4"][key]
			if want == nil && ok {
				return fmt.Errorf("stored %s = %v, want it removed", key, got)
			}
			if want != nil && got != want {
				return fmt.Errorf("stored %s = %v, want %v", key, got, want)
			}
			return nil
		}
	}
	inPlace := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(n, plancheck.ResourceActionUpdate),
	}}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: tun(`  proto     = "6in4"
  ip6prefix = ["2001:db8:46::/48"]
`),
				ResourceName:       n,
				ImportState:        true,
				ImportStateId:      "t6in4",
				ImportStatePersist: true,
			},
			{
				Config: tun(`  proto     = "6in4"
  ip6prefix = ["2001:db8:46::/48"]
  mtu       = 1400
`),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(n, "mtu", "1400"),
					stored("peeraddr", "192.0.2.1"),
				),
			},
			{
				Config: tun(`  proto = "6in4"
  mtu   = 1400
`),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(n, "ip6prefix.#"),
					stored("ip6prefix", nil),
					stored("peeraddr", "192.0.2.1"),
				),
			},
			{
				Config: tun(`  proto   = "static"
  ipaddrs = ["192.0.2.46"]
`),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(n, "proto", "static"),
					stored("peeraddr", nil),
				),
			},
		},
	})
}

// TestAccCreateCollisionHint covers the N2 UX: a create whose id collides with an
// existing section returns 422 (validation_failed + conflict field), and writeErr
// surfaces an actionable "terraform import" hint rather than a bare error.
func TestAccCreateCollisionHint(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	m.seedUnmanaged("/network/interfaces", "collide0", map[string]any{"proto": "static"})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_network_interface" "dup" {
  id    = "collide0"
  proto = "static"
}`,
			ExpectError: regexp.MustCompile("terraform import"),
		}},
	})
}

// TestAccDhcpHostDNSOnly covers the 2.2.0 relaxation: ip is no longer required, so
// a host can be a DNS-only reservation (macs + name, no static lease). Before 2.2.0
// the schema marked ip required and this config failed at plan.
func TestAccDhcpHostDNSOnly(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerHCL(m.URL) + `
resource "uapi_dhcp_host" "dnsonly" {
  id   = "dnsonly"
  name = "printer"
  macs = ["02:00:00:00:00:99"]
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("uapi_dhcp_host.dnsonly", "name", "printer"),
				resource.TestCheckResourceAttr("uapi_dhcp_host.dnsonly", "macs.0", "02:00:00:00:00:99"),
				resource.TestCheckNoResourceAttr("uapi_dhcp_host.dnsonly", "ip"),
			),
		}},
	})
}

// TestAccAdoptKeepsName is the P1 lockout regression: importing a pre-existing
// *named* section keeps its name (uapi >= 2.2.0 adopt-keep-name), so config with
// the same id reconciles with no replace. Before the fix this needed a ULID and
// then wanted a destroy+recreate.
func TestAccAdoptKeepsName(t *testing.T) {
	m := newMockUAPI()
	defer m.Close()
	m.seedUnmanaged("/dhcp/hosts", "printer", map[string]any{"ip": "192.168.9.9"})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: providerHCL(m.URL) + `
resource "uapi_dhcp_host" "printer" {
  id = "printer"
  ip = "192.168.9.9"
}`,
				ResourceName:       "uapi_dhcp_host.printer",
				ImportState:        true,
				ImportStateId:      "printer",
				ImportStatePersist: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if states[0].ID != "printer" {
						return fmt.Errorf("adopt-keep-name: id should stay \"printer\", got %q", states[0].ID)
					}
					return nil
				},
			},
			{
				Config: providerHCL(m.URL) + `
resource "uapi_dhcp_host" "printer" {
  id = "printer"
  ip = "192.168.9.9"
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
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

// A cron job has no update: the id is a hash of the line, so a change to either
// field is a new job. Import recovers both fields, unlike an authorized key.
func TestAccCronJob(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	cfg := func(command string) string {
		return providerCfg + fmt.Sprintf(`
resource "uapi_cron_job" "j" {
  schedule = "0 4 1 jan *"
  command  = %q
}

data "uapi_cron_job" "j" {
  id = uapi_cron_job.j.id
}`, command)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{
			{
				Config: cfg("true uapiacc-one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("uapi_cron_job.j", "id", regexp.MustCompile(`^[a-f0-9]{12}$`)),
					resource.TestCheckResourceAttr("uapi_cron_job.j", "schedule", "0 4 1 jan *"),
					resource.TestCheckResourceAttr("uapi_cron_job.j", "command", "true uapiacc-one"),
					resource.TestCheckResourceAttr("data.uapi_cron_job.j", "command", "true uapiacc-one"),
					resource.TestCheckResourceAttr("data.uapi_cron_job.j", "runtime.crond_running", "true"),
					resource.TestCheckResourceAttr("data.uapi_cron_job.j", "runtime.runs_as_written", "true"),
				),
			},
			{
				ResourceName:      "uapi_cron_job.j",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: cfg("true uapiacc-two"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("uapi_cron_job.j", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr("uapi_cron_job.j", "command", "true uapiacc-two"),
			},
		},
	})
}

// uapi answers 409 for a line the crontab already holds, which a second resource
// with the same schedule and command hits.
func TestAccCronJob_duplicate(t *testing.T) {
	providerCfg, closer := accTarget(t)
	defer closer()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: accProviders(),
		Steps: []resource.TestStep{{
			Config: providerCfg + `
resource "uapi_cron_job" "a" {
  schedule = "0 4 1 jan *"
  command  = "true uapiacc-dup"
}

resource "uapi_cron_job" "b" {
  schedule = "0 4 1 jan *"
  command  = "true uapiacc-dup"
}`,
			ExpectError: regexp.MustCompile(`already holds this schedule and command`),
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
  ipaddrs = ["192.168.1.1"]
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
				resource.TestCheckResourceAttr("data.uapi_diagnostics.d", "management_path.device", "br-lan"),
				// the sweep is opt-in, so it stays empty here
				resource.TestCheckResourceAttr("data.uapi_diagnostics.d", "invalid_sections.#", "0"),
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
  rate   = 50
  burst  = 100
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
	// rate/burst are write-only inputs the API never returns, so assert they
	// reached the wire via the captured create body rather than via state.
	if got := m.lastTokenCreate["rate"]; got != float64(50) {
		t.Errorf("rate not sent on mint: got %v, want 50", got)
	}
	if got := m.lastTokenCreate["burst"]; got != float64(100) {
		t.Errorf("burst not sent on mint: got %v, want 100", got)
	}
}
