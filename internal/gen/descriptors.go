package main

// descriptor is the hand-maintained overlay supplying what the OpenAPI spec
// cannot: nested (match) structure, kind, labels, and which data sources carry
// a runtime block. Field names/types/writeOnly/readOnly and required-ness are
// read from the spec (as of uapi 2.0 the curated schemas carry `required`).
type descriptor struct {
	Type          string
	Schema        string // OpenAPI components.schemas key
	Collection    string // path without leading slash (collection) or singleton path tail
	Kind          string // "collection" | "singleton"
	Label         string
	GenDataSource bool
	Nested        *nested
	Runtime       string   // "" | "interface" | "wireless": adds a computed runtime block to the data source
	CreateOnly    []string // fields that are create-time only and immutable (Optional + RequiresReplace, sent only on create, never returned), e.g. an interface `name`
}

// Desc returns a human description for a field (best-effort; docs only).
func (d descriptor) Desc(field string) string {
	if s, ok := commonDesc[field]; ok {
		return s
	}
	return "uci option " + field + "."
}

var commonDesc = map[string]string{
	"name":              "Optional section name.",
	"enabled":           "Whether the entry is active.",
	"disabled":          "Whether the entry is disabled.",
	"target":            "Target / action.",
	"proto":             "Protocol.",
	"family":            "Address family: any, ipv4, or ipv6.",
	"interface":         "Network interface this entry applies to.",
	"device":            "Underlying device.",
	"src_zone":          "Source firewall zone name.",
	"dest_zone":         "Destination firewall zone name.",
	"src_ip":            "Source IP addresses or CIDRs.",
	"dest_ip":           "Destination IP addresses or CIDRs.",
	"src_port":          "Source ports.",
	"dest_port":         "Destination ports.",
	"src_dport":         "Incoming (destination) ports to redirect.",
	"key":               "Encryption passphrase. Write-only: never returned by the API.",
	"private_key":       "WireGuard private key. Write-only: never returned by the API.",
	"preshared_key":     "WireGuard preshared key. Write-only: never returned by the API.",
	"has_key":           "Whether a key is configured (the value is never returned).",
	"has_private_key":   "Whether a private key is configured.",
	"has_preshared_key": "Whether a preshared key is configured.",
	"tls_auth":          "OpenVPN tls-auth/tls-crypt key material. Write-only: never returned by the API.",
	"pkcs12":            "OpenVPN PKCS#12 bundle. Write-only: never returned by the API.",
	"has_tls_auth":      "Whether tls-auth/tls-crypt key material is configured.",
	"has_pkcs12":        "Whether a PKCS#12 bundle is configured.",
	"probe_count":       "Probes per cycle per track_ip.",
	"download":          "Download shaping rate in kbit/s.",
	"upload":            "Upload shaping rate in kbit/s.",
}

func matchFields(redirect bool) *nested {
	f := []field{
		{Name: "src_zone", GoName: "SrcZone", GoType: "types.String", Kind: "required", Desc: "Source firewall zone name."},
		{Name: "dest_zone", GoName: "DestZone", GoType: "types.String", Kind: "optcomp", Desc: "Destination firewall zone name."},
		{Name: "src_ip", GoName: "SrcIP", GoType: "types.List", Kind: "optcomp", Desc: "Source IP addresses or CIDRs."},
		{Name: "src_port", GoName: "SrcPort", GoType: "types.List", Kind: "optcomp", Desc: "Source ports."},
	}
	if redirect {
		f = append(f, field{Name: "src_dport", GoName: "SrcDport", GoType: "types.List", Kind: "optcomp", Desc: "Incoming (destination) ports to redirect."})
		f = append(f, field{Name: "dest_ip", GoName: "DestIP", GoType: "types.List", Kind: "optcomp", Desc: "Internal destination IP addresses."})
		f = append(f, field{Name: "dest_port", GoName: "DestPort", GoType: "types.List", Kind: "optcomp", Desc: "Internal destination ports."})
	} else {
		f = append(f, field{Name: "dest_ip", GoName: "DestIP", GoType: "types.List", Kind: "optcomp", Desc: "Destination IP addresses or CIDRs."})
		f = append(f, field{Name: "dest_port", GoName: "DestPort", GoType: "types.List", Kind: "optcomp", Desc: "Destination ports."})
	}
	f = append(f,
		field{Name: "proto", GoName: "Proto", GoType: "types.List", Kind: "optcomp", Desc: "Protocols."},
		field{Name: "family", GoName: "Family", GoType: "types.String", Kind: "optcomp", Desc: "Address family: any, ipv4, or ipv6."},
	)
	gt := "firewallRuleMatch"
	if redirect {
		gt = "firewallRedirectMatch"
	}
	return &nested{Name: "match", GoType: gt, Fields: f}
}

var descriptors = []descriptor{
	// firewall
	{Type: "firewall_zone", Schema: "FirewallZones", Collection: "firewall/zones", Kind: "collection", Label: "firewall zone", GenDataSource: true},
	{Type: "firewall_rule", Schema: "FirewallRules", Collection: "firewall/rules", Kind: "collection", Label: "firewall rule", GenDataSource: true, Nested: matchFields(false)},
	{Type: "firewall_redirect", Schema: "FirewallRedirects", Collection: "firewall/redirects", Kind: "collection", Label: "firewall redirect", GenDataSource: true, Nested: matchFields(true)},
	{Type: "firewall_forwarding", Schema: "FirewallForwardings", Collection: "firewall/forwardings", Kind: "collection", Label: "firewall forwarding", GenDataSource: true},
	{Type: "firewall_defaults", Schema: "FirewallDefaults", Collection: "firewall/defaults", Kind: "singleton", Label: "firewall defaults", GenDataSource: true},
	// network (interface + wireless_interface data sources are hand-written: runtime)
	{Type: "network_interface", Schema: "NetworkInterfaces", Collection: "network/interfaces", Kind: "collection", Label: "network interface", GenDataSource: true, Runtime: "interface", CreateOnly: []string{"name"}},
	{Type: "network_device", Schema: "NetworkDevices", Collection: "network/devices", Kind: "collection", Label: "network device", GenDataSource: true},
	{Type: "network_route", Schema: "NetworkRoutes", Collection: "network/routes", Kind: "collection", Label: "network route", GenDataSource: true},
	{Type: "network_rule", Schema: "NetworkRules", Collection: "network/rules", Kind: "collection", Label: "network rule", GenDataSource: true},
	{Type: "network_bridge_vlan", Schema: "NetworkBridgeVlans", Collection: "network/bridge_vlans", Kind: "collection", Label: "network bridge VLAN", GenDataSource: true},
	{Type: "network_wireguard_peer", Schema: "NetworkWireguardPeers", Collection: "network/wireguard_peers", Kind: "collection", Label: "network WireGuard peer", GenDataSource: true},
	// wireless
	{Type: "wireless_device", Schema: "WirelessDevices", Collection: "wireless/devices", Kind: "collection", Label: "wireless device", GenDataSource: true},
	{Type: "wireless_interface", Schema: "WirelessInterfaces", Collection: "wireless/interfaces", Kind: "collection", Label: "wireless interface", GenDataSource: true, Runtime: "wireless"},
	// dhcp
	{Type: "dhcp_host", Schema: "DhcpHosts", Collection: "dhcp/hosts", Kind: "collection", Label: "dhcp host", GenDataSource: true},
	{Type: "dhcp_server", Schema: "DhcpServers", Collection: "dhcp/servers", Kind: "collection", Label: "dhcp server", GenDataSource: true},
	{Type: "dhcp_dnsmasq", Schema: "DhcpDnsmasq", Collection: "dhcp/dnsmasq", Kind: "singleton", Label: "dnsmasq settings", GenDataSource: true},
	{Type: "dhcp_odhcpd", Schema: "DhcpOdhcpd", Collection: "dhcp/odhcpd", Kind: "singleton", Label: "odhcpd settings", GenDataSource: true},
	// snmpd
	{Type: "snmpd_access", Schema: "SnmpdAccesses", Collection: "snmpd/accesses", Kind: "collection", Label: "snmpd access", GenDataSource: true},
	{Type: "snmpd_agent", Schema: "SnmpdAgents", Collection: "snmpd/agents", Kind: "collection", Label: "snmpd agent", GenDataSource: true},
	{Type: "snmpd_com2sec", Schema: "SnmpdCom2secs", Collection: "snmpd/com2secs", Kind: "collection", Label: "snmpd com2sec", GenDataSource: true},
	{Type: "snmpd_group", Schema: "SnmpdGroups", Collection: "snmpd/groups", Kind: "collection", Label: "snmpd group", GenDataSource: true},
	{Type: "snmpd_system", Schema: "SnmpdSystem", Collection: "snmpd/system", Kind: "singleton", Label: "snmpd system", GenDataSource: true},
	// sqm / uhttpd / dropbear / system / vnstat / unbound / lldpd / prometheus
	{Type: "sqm_queue", Schema: "SqmQueues", Collection: "sqm/queues", Kind: "collection", Label: "sqm queue", GenDataSource: true},
	{Type: "uhttpd_cert", Schema: "UhttpdCerts", Collection: "uhttpd/certs", Kind: "collection", Label: "uhttpd cert", GenDataSource: true},
	{Type: "uhttpd_instance", Schema: "UhttpdInstances", Collection: "uhttpd/instances", Kind: "collection", Label: "uhttpd instance", GenDataSource: true},
	{Type: "dropbear_instance", Schema: "DropbearInstances", Collection: "dropbear/instances", Kind: "collection", Label: "dropbear instance", GenDataSource: true},
	{Type: "system_timeserver", Schema: "SystemTimeservers", Collection: "system/timeservers", Kind: "collection", Label: "system timeserver", GenDataSource: true},
	{Type: "vnstat_interface", Schema: "VnstatInterfaces", Collection: "vnstat/interfaces", Kind: "collection", Label: "vnstat interface", GenDataSource: true},
	{Type: "system", Schema: "System", Collection: "system", Kind: "singleton", Label: "system settings", GenDataSource: true},
	{Type: "unbound_server", Schema: "UnboundServer", Collection: "unbound/server", Kind: "singleton", Label: "unbound server", GenDataSource: true},
	{Type: "vnstat_config", Schema: "VnstatConfig", Collection: "vnstat/config", Kind: "singleton", Label: "vnstat config", GenDataSource: true},
	{Type: "lldpd_config", Schema: "LldpdConfig", Collection: "lldpd/config", Kind: "singleton", Label: "lldpd config", GenDataSource: true},
	{Type: "prometheus_node_exporter_lua_config", Schema: "PrometheusNodeExporterLuaConfig", Collection: "prometheus_node_exporter_lua/config", Kind: "singleton", Label: "prometheus node_exporter config", GenDataSource: true},
	// mwan3 (added in uapi 2.0.0-rc3)
	{Type: "mwan3_interface", Schema: "Mwan3Interfaces", Collection: "mwan3/interfaces", Kind: "collection", Label: "mwan3 interface", GenDataSource: true},
	{Type: "mwan3_member", Schema: "Mwan3Members", Collection: "mwan3/members", Kind: "collection", Label: "mwan3 member", GenDataSource: true},
	{Type: "mwan3_policy", Schema: "Mwan3Policies", Collection: "mwan3/policies", Kind: "collection", Label: "mwan3 policy", GenDataSource: true},
	{Type: "mwan3_rule", Schema: "Mwan3Rules", Collection: "mwan3/rules", Kind: "collection", Label: "mwan3 rule", GenDataSource: true},
	{Type: "mwan3_globals", Schema: "Mwan3Globals", Collection: "mwan3/globals", Kind: "singleton", Label: "mwan3 globals", GenDataSource: true},
	// usteer + openvpn (added in uapi 2.0.0-rc3; openvpn key/tls_auth/pkcs12 are write-only per the spec)
	{Type: "usteer_config", Schema: "UsteerConfig", Collection: "usteer/config", Kind: "singleton", Label: "usteer config", GenDataSource: true},
	{Type: "openvpn_instance", Schema: "OpenvpnInstances", Collection: "openvpn/instances", Kind: "collection", Label: "openvpn instance", GenDataSource: true},
}
