package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAllResources creates every resource type with a minimal config against
// a fresh fake, asserting it gets a stable id (and is managed, for uci
// resources). The framework's post-apply empty-plan check also proves
// idempotency for each. Import/update paths are covered by the pattern tests.
func TestAccAllResources(t *testing.T) {
	cases := []struct {
		typ       string
		hcl       string
		singleton bool // skip the managed=true check
		noEtag    bool // packages carry no etag (replace-only, non-uci)
	}{
		// firewall
		{typ: "uapi_firewall_zone", hcl: `resource "uapi_firewall_zone" "t" {
  name = "z"
}`},
		{typ: "uapi_firewall_rule", hcl: `resource "uapi_firewall_rule" "t" {
  target = "ACCEPT"
  match  = { src_zone = "wan" }
}`},
		{typ: "uapi_firewall_redirect", hcl: `resource "uapi_firewall_redirect" "t" {
  match = { src_zone = "wan" }
}`},
		{typ: "uapi_firewall_forwarding", hcl: `resource "uapi_firewall_forwarding" "t" {
  src  = "lan"
  dest = "wan"
}`},
		{typ: "uapi_firewall_nat", hcl: `resource "uapi_firewall_nat" "t" {
  target = "MASQUERADE"
  match  = { src_zone = "wan" }
}`},
		{typ: "uapi_firewall_defaults", singleton: true, hcl: `resource "uapi_firewall_defaults" "t" {
  input = "ACCEPT"
}`},
		// network
		{typ: "uapi_network_interface", hcl: `resource "uapi_network_interface" "t" {
  proto = "dhcp"
}`},
		{typ: "uapi_network_device", hcl: `resource "uapi_network_device" "t" {
  name  = "br0"
  type  = "bridge"
  ports = ["lan1"]
}`},
		{typ: "uapi_network_route", hcl: `resource "uapi_network_route" "t" {
  target = "10.9.0.0/24"
}`},
		{typ: "uapi_network_rule", hcl: `resource "uapi_network_rule" "t" {
  src    = "192.168.9.0/24"
  lookup = 1
}`},
		// Deliberately NOT br-lan. Creating a bridge VLAN on the management bridge
		// enables VLAN filtering on it, untagged traffic stops, and a live run takes
		// its own target off the network mid-suite (the delete then never runs, so
		// the section stays committed in uci).
		//
		// It creates the bridge it filters, rather than naming one it hopes exists.
		// Naming a nonexistent device made the case fail every live run with
		// `device: conflict, bridge "br-uapitest" does not exist`, which is a
		// permanently red test nobody reads. A portless bridge (uapi >= 2.2.0)
		// carries no traffic, so filtering it cannot strand anything.
		{typ: "uapi_network_bridge_vlan", hcl: `resource "uapi_network_device" "for_vlan" {
  name = "br-uapitest"
  type = "bridge"
}

resource "uapi_network_bridge_vlan" "t" {
  device = uapi_network_device.for_vlan.name
  vlan   = 9
}`},
		// Creates the wireguard interface it attaches to. Naming a bare "wg0" assumed
		// one already existed, which no real router guarantees, so this shared the
		// defect fixed in the snmpd_access and sqm_queue cases.
		//
		// `private_key` is a throwaway generated for this fixture, not a secret: the
		// interface exists for the length of one test. It has to be a real 44-char
		// (32-byte) base64 key, and `addresses` is required when proto is wireguard.
		// The fake validates neither, so the first version of this case carried a
		// 48-char string and no addresses and was refused by every real router.
		{typ: "uapi_network_wireguard_peer", hcl: `resource "uapi_network_interface" "for_peer" {
  id          = "wgfixture"
  proto       = "wireguard"
  private_key = "4TMrf9VJAdq0+HDQRFG9uuRYXTtMmu6PrCwpaomeICk="
  listen_port = 51999
  addresses   = ["10.9.0.1/24"]
}

resource "uapi_network_wireguard_peer" "t" {
  interface   = uapi_network_interface.for_peer.id
  public_key  = "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg="
  allowed_ips = ["10.9.0.2/32"]
}`},
		// wireless
		{typ: "uapi_wireless_device", hcl: `resource "uapi_wireless_device" "t" {
  type = "mac80211"
}`},
		{typ: "uapi_wireless_interface", hcl: `resource "uapi_wireless_interface" "t" {
  device = "radio0"
}`},
		// dhcp / dns
		{typ: "uapi_dhcp_host", hcl: `resource "uapi_dhcp_host" "t" {
  ip   = "192.168.9.2"
  macs = ["02:00:00:00:00:09"]
}`},
		{typ: "uapi_dhcp_server", hcl: `resource "uapi_dhcp_server" "t" {
  interface = "lan"
}`},
		{typ: "uapi_dhcp_dnsmasq", singleton: true, hcl: `resource "uapi_dhcp_dnsmasq" "t" {
  domain = "lan"
}`},
		{typ: "uapi_dhcp_odhcpd", singleton: true, hcl: `resource "uapi_dhcp_odhcpd" "t" {}`},
		{typ: "uapi_unbound_server", singleton: true, hcl: `resource "uapi_unbound_server" "t" {}`},
		// snmpd
		// Creates the group it references. Referencing a bare name assumed a group
		// already existed, which is true of the fake and of no real router, so this
		// case failed every live run and had to be triaged as known-noise.
		{typ: "uapi_snmpd_access", hcl: `resource "uapi_snmpd_group" "for_access" {
  group = "acc_group"
}

resource "uapi_snmpd_access" "t" {
  group = uapi_snmpd_group.for_access.group
}`},
		{typ: "uapi_snmpd_agent", hcl: `resource "uapi_snmpd_agent" "t" {}`},
		{typ: "uapi_snmpd_com2sec", hcl: `resource "uapi_snmpd_com2sec" "t" {
  secname   = "ro"
  source    = "default"
  community = "public"
}`},
		{typ: "uapi_snmpd_group", hcl: `resource "uapi_snmpd_group" "t" {
  group = "g"
}`},
		{typ: "uapi_snmpd_system", singleton: true, hcl: `resource "uapi_snmpd_system" "t" {}`},
		// uhttpd / dropbear / sqm / system / vnstat / lldpd / prometheus
		{typ: "uapi_uhttpd_cert", hcl: `resource "uapi_uhttpd_cert" "t" {
  commonname = "router"
}`},
		{typ: "uapi_uhttpd_instance", hcl: `resource "uapi_uhttpd_instance" "t" {}`},
		{typ: "uapi_dropbear_instance", hcl: `resource "uapi_dropbear_instance" "t" {}`},
		// `wan` exists on a stock OpenWrt where `eth1` does not, and enabled=false
		// keeps sqm from actually shaping the interface a live run may be arriving
		// over. Exercises create/read/destroy without touching traffic.
		{typ: "uapi_sqm_queue", hcl: `resource "uapi_sqm_queue" "t" {
  interface = "wan"
  enabled   = false
}`},
		{typ: "uapi_system_timeserver", hcl: `resource "uapi_system_timeserver" "t" {}`},
		{typ: "uapi_vnstat_config", singleton: true, hcl: `resource "uapi_vnstat_config" "t" {}`},
		{typ: "uapi_lldpd_config", singleton: true, hcl: `resource "uapi_lldpd_config" "t" {}`},
		{typ: "uapi_prometheus_node_exporter_lua_config", singleton: true, hcl: `resource "uapi_prometheus_node_exporter_lua_config" "t" {}`},
		// mwan3 / usteer / openvpn (rc3)
		{typ: "uapi_mwan3_interface", hcl: `resource "uapi_mwan3_interface" "t" {
  family      = "ipv4"
  probe_count = 3
}`},
		{typ: "uapi_mwan3_member", hcl: `resource "uapi_mwan3_member" "t" {
  interface = "wan"
}`},
		{typ: "uapi_mwan3_policy", hcl: `resource "uapi_mwan3_policy" "t" {
  use_members = ["wan_m1_w3"]
}`},
		{typ: "uapi_mwan3_rule", hcl: `resource "uapi_mwan3_rule" "t" {
  use_policy = "wan_only"
}`},
		{typ: "uapi_mwan3_globals", singleton: true, hcl: `resource "uapi_mwan3_globals" "t" {}`},
		{typ: "uapi_usteer_config", singleton: true, hcl: `resource "uapi_usteer_config" "t" {}`},
		// `key` is a path on the router, not key material. The fixture used to pass
		// a PEM header, which the fake accepted and a real router rejects with
		// "must be an absolute filesystem path".
		{typ: "uapi_openvpn_instance", hcl: `resource "uapi_openvpn_instance" "t" {
  client = true
  key    = "/etc/openvpn/client.key"
}`},
		// packages (no etag)
		{typ: "uapi_package", noEtag: true, hcl: `resource "uapi_package" "t" {
  name = "curl"
}`},
		{typ: "uapi_package_feed", noEtag: true, hcl: `resource "uapi_package_feed" "t" {
  name = "custom"
  url  = "http://example.test/feed"
}`},
	}

	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			providerCfg, closer := accTarget(t)
			defer closer()
			addr := c.typ + ".t"
			checks := []resource.TestCheckFunc{
				resource.TestCheckResourceAttrSet(addr, "id"),
			}
			if !c.noEtag {
				checks = append(checks, resource.TestCheckResourceAttrSet(addr, "etag"))
			}
			if !c.singleton {
				checks = append(checks, resource.TestCheckResourceAttr(addr, "managed", "true"))
			}
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: accProviders(),
				Steps: []resource.TestStep{{
					Config: providerCfg + "\n" + c.hcl,
					Check:  resource.ComposeAggregateTestCheckFunc(checks...),
				}},
			})
		})
	}
}
