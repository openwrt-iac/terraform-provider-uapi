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
		{typ: "uapi_firewall_defaults", singleton: true, hcl: `resource "uapi_firewall_defaults" "t" {
  input = "ACCEPT"
}`},
		// network
		{typ: "uapi_network_interface", hcl: `resource "uapi_network_interface" "t" {
  proto = "dhcp"
}`},
		{typ: "uapi_network_device", hcl: `resource "uapi_network_device" "t" {
  name = "br0"
  type = "bridge"
}`},
		{typ: "uapi_network_route", hcl: `resource "uapi_network_route" "t" {
  target = "10.9.0.0/24"
}`},
		{typ: "uapi_network_rule", hcl: `resource "uapi_network_rule" "t" {
  src = "192.168.9.0/24"
}`},
		{typ: "uapi_network_bridge_vlan", hcl: `resource "uapi_network_bridge_vlan" "t" {
  device = "br-lan"
  vlan   = "9"
}`},
		{typ: "uapi_network_wireguard_peer", hcl: `resource "uapi_network_wireguard_peer" "t" {
  interface   = "wg0"
  public_key  = "k"
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
  ip  = "192.168.9.2"
  mac = "02:00:00:00:00:09"
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
		{typ: "uapi_snmpd_access", hcl: `resource "uapi_snmpd_access" "t" {
  group = "g"
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
		{typ: "uapi_sqm_queue", hcl: `resource "uapi_sqm_queue" "t" {
  interface = "eth1"
}`},
		{typ: "uapi_system_timeserver", hcl: `resource "uapi_system_timeserver" "t" {}`},
		{typ: "uapi_vnstat_interface", hcl: `resource "uapi_vnstat_interface" "t" {
  interface = "eth0"
}`},
		{typ: "uapi_vnstat_config", singleton: true, hcl: `resource "uapi_vnstat_config" "t" {}`},
		{typ: "uapi_lldpd_config", singleton: true, hcl: `resource "uapi_lldpd_config" "t" {}`},
		{typ: "uapi_prometheus_node_exporter_lua_config", singleton: true, hcl: `resource "uapi_prometheus_node_exporter_lua_config" "t" {}`},
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
			m := newMockUAPI()
			defer m.Close()
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
					Config: providerHCL(m.URL) + "\n" + c.hcl,
					Check:  resource.ComposeAggregateTestCheckFunc(checks...),
				}},
			})
		})
	}
}
