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
	Descs         map[string]string
	// SchemaVersion is emitted as the schema's Version. Bump it when an
	// attribute's Terraform type changes shape (list <-> string), which makes
	// prior state undecodable, and add the matching StateUpgrader in
	// state_upgrades.go. Adding or removing an attribute does not need one.
	SchemaVersion int
}

// Desc returns a human description for a field (best-effort; docs only). The
// per-resource Descs win over commonDesc, which is keyed by field name alone and
// so cannot describe a field whose meaning differs per resource (`target`).
func (d descriptor) Desc(field string) string {
	if s, ok := d.Descs[field]; ok {
		return s
	}
	if s, ok := commonDesc[field]; ok {
		return s
	}
	return "uci option " + field + "."
}

// Shared by the three firewall match shapes, which validate these identically.
const (
	protoDesc = "Protocols to match, by name or number (`tcp`, `udp`, `gre`, `sctp`, `47`) or a wildcard (`all`, `any`, `tcpudp`). Every protocol must be tcp or udp when a port is matched, because firewall4 keeps a port match only on those."
	markDesc  = "Match an fwmark as a value or value/mask, decimal or `0x` hex. Prefix with `!` to negate."
)

var commonDesc = map[string]string{
	"name":               "Optional section name.",
	"enabled":            "Whether the entry is active.",
	"disabled":           "Whether the entry is disabled.",
	"target":             "Target / action.",
	"proto":              "Protocol.",
	"family":             "Address family: any, ipv4, or ipv6.",
	"interface":          "Network interface this entry applies to.",
	"device":             "Underlying device.",
	"src_zone":           "Source firewall zone name.",
	"dest_zone":          "Destination firewall zone name.",
	"src_ip":             "Source IP addresses or CIDRs.",
	"dest_ip":            "Destination IP addresses or CIDRs.",
	"src_port":           "Source ports.",
	"dest_port":          "Destination ports.",
	"src_dport":          "Incoming (destination) ports to redirect.",
	"set_mark":           "fwmark to set, as a value or value/mask (decimal or `0x` hex). Target MARK requires this or `set_xmark`.",
	"set_xmark":          "fwmark to set with XOR semantics, as a value or value/mask. The alternative to `set_mark` for target MARK.",
	"set_dscp":           "DSCP class (`CS0` to `CS7`, `BE`, `LE`, `AF11` to `AF43`, `EF`, case-insensitive) or value 0-63 to set. Required by target DSCP.",
	"snat_ip":            "IPv4 address to rewrite the source to. Requires target SNAT.",
	"snat_port":          "Port or port range to rewrite the source port to. Requires target SNAT.",
	"key":                "Encryption passphrase. Write-only: never returned by the API.",
	"private_key":        "WireGuard private key. Write-only: never returned by the API.",
	"preshared_key":      "WireGuard preshared key. Write-only: never returned by the API.",
	"has_key":            "Whether a key is configured (the value is never returned).",
	"has_private_key":    "Whether a private key is configured.",
	"has_preshared_key":  "Whether a preshared key is configured.",
	"tls_auth":           "OpenVPN tls-auth/tls-crypt key material. Write-only: never returned by the API.",
	"pkcs12":             "OpenVPN PKCS#12 bundle. Write-only: never returned by the API.",
	"has_tls_auth":       "Whether tls-auth/tls-crypt key material is configured.",
	"has_pkcs12":         "Whether a PKCS#12 bundle is configured.",
	"probe_count":        "Probes per cycle per track_ip.",
	"download":           "Download shaping rate in kbit/s.",
	"upload":             "Upload shaping rate in kbit/s.",
	"ip_transparent":     "Bind to addresses that are not yet up (VIP / alias / floating addresses).",
	"interface_bind":     "Addresses unbound binds on (`addr` or `addr@port`). Pair with the main unbound `interface_auto = false` for exclusive binding.",
	"interface_outgoing": "Source addresses for upstream recursion (multi-WAN egress).",
	"srv_line":           "Verbatim lines inserted inside unbound's `server:` clause. One entry per line; `unbound-checkconf` validates grammar after restart.",
	"ext_line":           "Verbatim lines rendered into unbound_ext.conf, outside the `server:` clause. One entry per line; build whole `forward-zone:`/`view:`/`stub:` clauses by listing them in order.",
}

// A rule needs no source zone (fw4 puts it in the output chain), but a redirect
// always does, so src_zone is required on one and not the other.
func matchFields(redirect bool) *nested {
	srcZone := field{Name: "src_zone", GoName: "SrcZone", GoType: "types.String", Kind: "optcomp",
		Desc: "Source firewall zone name. Omit it for an output-chain rule; target NOTRACK requires a real zone name here (not the `*` wildcard)."}
	if redirect {
		srcZone.Kind = "required"
		srcZone.Desc = "Source firewall zone name."
	}
	// A rule matches with real lists. A redirect's selectors are scalars as of uapi
	// 3.0: firewall4 parses a `config redirect` option as a scalar, and the array
	// with maxItems 1 that v2 used made a second value an apply-time 422 instead of
	// a plan-time type error.
	addrType, portType := "types.List", "types.List"
	if redirect {
		addrType, portType = "types.String", "types.String"
	}
	f := []field{
		srcZone,
		{Name: "dest_zone", GoName: "DestZone", GoType: "types.String", Kind: "optcomp", Desc: "Destination firewall zone name."},
		{Name: "src_ip", GoName: "SrcIP", GoType: addrType, Kind: "optcomp", Desc: "Source IP addresses or CIDRs."},
		{Name: "src_port", GoName: "SrcPort", GoType: portType, Kind: "optcomp", Desc: "Source ports."},
	}
	if redirect {
		f = append(f, field{Name: "src_dport", GoName: "SrcDport", GoType: portType, Kind: "optcomp", Desc: "With target DNAT, the incoming (destination) port or range to redirect. With target SNAT, the source port to rewrite to."})
		f = append(f, field{Name: "src_dip", GoName: "SrcDip", GoType: addrType, Kind: "optcomp", Desc: "With target DNAT, the external destination address to match, which also selects the address used for NAT reflection. With target SNAT, the address to rewrite the source to, and required."})
		f = append(f, field{Name: "dest_ip", GoName: "DestIP", GoType: addrType, Kind: "optcomp", Desc: "Internal destination address to rewrite to."})
		f = append(f, field{Name: "dest_port", GoName: "DestPort", GoType: portType, Kind: "optcomp", Desc: "Internal destination port or range to rewrite to."})
	} else {
		f = append(f, field{Name: "dest_ip", GoName: "DestIP", GoType: addrType, Kind: "optcomp", Desc: "Destination IP addresses or CIDRs."})
		f = append(f, field{Name: "dest_port", GoName: "DestPort", GoType: portType, Kind: "optcomp", Desc: "Destination ports."})
	}
	f = append(f,
		field{Name: "proto", GoName: "Proto", GoType: "types.List", Kind: "optcomp", Desc: protoDesc},
		field{Name: "family", GoName: "Family", GoType: "types.String", Kind: "optcomp", Desc: "Address family: any, ipv4, or ipv6."},
		field{Name: "mark", GoName: "Mark", GoType: "types.String", Kind: "optcomp", Desc: markDesc},
	)
	if !redirect {
		f = append(f, field{Name: "dscp", GoName: "Dscp", GoType: "types.String", Kind: "optcomp", Desc: "Match a DSCP class (`CS0` to `CS7`, `BE`, `LE`, `AF11` to `AF43`, `EF`, case-insensitive) or a value 0-63. Prefix with `!` to negate."})
	}
	gt := "firewallRuleMatch"
	if redirect {
		gt = "firewallRedirectMatch"
	}
	return &nested{Name: "match", GoType: gt, Fields: f}
}

// firewall/nat is a third match shape: firewall4 parses the addresses and ports
// of a `config nat` section as scalars, so these are strings where the rule and
// redirect equivalents are lists.
func natMatchFields() *nested {
	return &nested{Name: "match", GoType: "firewallNatMatch", Fields: []field{
		{Name: "src_zone", GoName: "SrcZone", GoType: "types.String", Kind: "optcomp", Desc: "Outbound (postrouting) zone this rule applies to. Unset matches all egress traffic."},
		{Name: "device", GoName: "Device", GoType: "types.String", Kind: "optcomp", Desc: "Outbound interface name to match."},
		{Name: "src_ip", GoName: "SrcIP", GoType: "types.String", Kind: "optcomp", Desc: "Source address to match: an address, a prefix in either family, or a uci network name. Prefix with `!` to negate."},
		{Name: "src_port", GoName: "SrcPort", GoType: "types.String", Kind: "optcomp", Desc: "Source port or range to match. Prefix with `!` to negate."},
		{Name: "dest_ip", GoName: "DestIP", GoType: "types.String", Kind: "optcomp", Desc: "Destination address to match, in the same forms as `src_ip`."},
		{Name: "dest_port", GoName: "DestPort", GoType: "types.String", Kind: "optcomp", Desc: "Destination port or range to match. Prefix with `!` to negate."},
		{Name: "proto", GoName: "Proto", GoType: "types.List", Kind: "optcomp", Desc: protoDesc + " Defaults to all when unset."},
		{Name: "mark", GoName: "Mark", GoType: "types.String", Kind: "optcomp", Desc: markDesc},
		{Name: "family", GoName: "Family", GoType: "types.String", Kind: "optcomp", Desc: "Address family: any, ipv4, or ipv6. Unset means IPv4 only, firewall4's backwards-compatible default for NAT; set `any` for dual-stack."},
	}}
}

var descriptors = []descriptor{
	// firewall
	{Type: "firewall_zone", Schema: "FirewallZones", Collection: "firewall/zones", Kind: "collection", Label: "firewall zone", GenDataSource: true, Descs: map[string]string{
		"name": "Zone name, referenced by rules and forwardings. Alphanumerics, dashes and underscores.",
	}},
	{Type: "firewall_rule", Schema: "FirewallRules", Collection: "firewall/rules", Kind: "collection", Label: "firewall rule", GenDataSource: true, Nested: matchFields(false)},
	{Type: "firewall_redirect", Schema: "FirewallRedirects", Collection: "firewall/redirects", Kind: "collection", Label: "firewall redirect", GenDataSource: true, Nested: matchFields(true), SchemaVersion: 1},
	{Type: "firewall_forwarding", Schema: "FirewallForwardings", Collection: "firewall/forwardings", Kind: "collection", Label: "firewall forwarding", GenDataSource: true},
	{Type: "firewall_nat", Schema: "FirewallNat", Collection: "firewall/nat", Kind: "collection", Label: "firewall NAT rule", GenDataSource: true, Nested: natMatchFields(), Descs: map[string]string{
		"target": "What to do with matched traffic: `SNAT` rewrites the source to `snat_ip`/`snat_port`, `MASQUERADE` rewrites it to the outbound interface address, `ACCEPT` exempts it from source NAT. Defaults to `MASQUERADE`.",
		"name":   "Human-readable label for this NAT rule.",
	}},
	{Type: "firewall_defaults", Schema: "FirewallDefaults", Collection: "firewall/defaults", Kind: "singleton", Label: "firewall defaults", GenDataSource: true},
	// network (interface + wireless_interface data sources are hand-written: runtime)
	{Type: "network_interface", Schema: "NetworkInterfaces", Collection: "network/interfaces", Kind: "collection", Label: "network interface", GenDataSource: true, Runtime: "interface", SchemaVersion: 1,
		// Phrased to read correctly on the data source too, where "set one or the
		// other" would be meaningless: these state a fact about the API rather than
		// instructing the reader.
		// The generator appends "Applies only to proto ..." to each field that the
		// spec scopes, so these leave the protos out.
		Descs: map[string]string{
			"ipaddr":         "Static IPv4 address, read-only: the single-address view of the first `ipaddrs` entry. Both names are one uci option (`list ipaddr`), and as of uapi 3.0 only `ipaddrs` is writable. Set `ipaddrs` even for a single address.",
			"ipaddrs":        "Static IPv4 addresses (uci `list ipaddr`). The only writable form as of uapi 3.0; `ipaddr` is a read-only view of the first entry.",
			"proto":          "Interface protocol: `static`, `dhcp`, `dhcpv6`, `pppoe`, `ppp`, `wwan`, `wireguard` or `none`. When it changes, the plan sets to null each attribute that the new proto does not take. uapi also reads a proto that it does not model, such as a `6in4` tunnel from LuCI. An update of such an interface is a PATCH of the changed attributes, which keeps the options of its handler. uapi refuses a switch to such a proto.",
			"broadcast":      "IPv4 broadcast address. On proto `dhcp`, the same uci option is a flag: `broadcast_flag`.",
			"broadcast_flag": "Whether udhcpc asks the DHCP server to broadcast its replies (`-B`), as LuCI's \"Use broadcast flag\". uci stores it as `broadcast`, which is the broadcast address on proto `static`. Unset means false. Needs uapi 3.2.0 or later.",
			"clientid":       "Client identifier: DHCP option 61 on proto `dhcp`, a DUID on proto `dhcpv6`.",
			"reqprefix":      "Prefix delegation request: `auto`, `no`, a prefix length such as `56`, or a prefix with its length such as `2001:db8::/56`.",
			"sourcefilter":   "Whether the default route from upstream is source-specific. Unset means true. A routed `ip6prefix` needs false: otherwise traffic from its downstream subnets matches no default route. Needs uapi 3.2.0 or later.",
			"listen_port":    "UDP port that WireGuard listens on. Null or 0 lets the kernel pick one. A service name stored in uci reads as its port, and -1 reads when the name does not resolve. uapi refuses a write of -1, so set a port.",
			"ip6prefix":      "IPv6 prefixes routed to this device, for delegation to clients. uapi stores one prefix as a uci option and several as a uci list. A list since provider 3.2.0, which needs uapi 3.2.0 or later.",
			"mtu":            "MTU that netifd applies to the devices of this interface, from 68. A device with its own `config device` section keeps its own MTU: set it on `uapi_network_device`. On proto `ppp` and `pppoe`, pppd takes 128 to 16384.",
			"metric":         "Metric of the routes that this interface installs without a metric of their own. Applies to every proto.",
			"defaultroute":   "Whether to install the default route that the protocol provides. Unset means true. Applies to every proto.",
			"delegate":       "Whether to hand the IPv6 prefixes of this interface on to downstream interfaces. Unset means true. Applies to every proto.",
			"ip4table":       "Routing table for the IPv4 routes that this interface installs: a number, or a name from `/etc/iproute2/rt_tables`. Applies to every proto.",
			"ip6table":       "Routing table for the IPv6 routes that this interface installs: a number, or a name from `/etc/iproute2/rt_tables`. Applies to every proto.",
			"ip6assign":      "Length of the prefix that this interface takes from a delegated or ULA prefix, such as `64`. Applies to every proto.",
			"ip6hint":        "Hexadecimal subnet id that selects the part of a delegated or ULA prefix that this interface gets, such as `1` or `0x1f`. netifd uses it only when `ip6assign` is set. Applies to every proto.",
			"ip6class":       "Prefix classes that this interface can take its subnet from: `local` for the ULA prefix, or the name of the interface that received the prefix. Unset means any. Needs uapi 3.2.0 or later.",
			"ip6weight":      "Assignment preference: when interfaces share a prefix, the interfaces with a higher weight get their subnet first. Needs uapi 3.2.0 or later.",
		}},
	{Type: "network_device", Schema: "NetworkDevices", Collection: "network/devices", Kind: "collection", Label: "network device", GenDataSource: true, Descs: map[string]string{
		"name": "Device name as netifd and the kernel see it (`br-lan`, `eth0`).",
	}},
	{Type: "network_route", Schema: "NetworkRoutes", Collection: "network/routes", Kind: "collection", Label: "network route", GenDataSource: true, Descs: map[string]string{
		"target":    "Destination IPv4 address or CIDR for this route.",
		"interface": "Parent network interface name.",
	}},
	{Type: "network_rule", Schema: "NetworkRules", Collection: "network/rules", Kind: "collection", Label: "network rule", GenDataSource: true},
	{Type: "network_bridge_vlan", Schema: "NetworkBridgeVlans", Collection: "network/bridge_vlans", Kind: "collection", Label: "network bridge VLAN", GenDataSource: true},
	{Type: "network_wireguard_peer", Schema: "NetworkWireguardPeers", Collection: "network/wireguard_peers", Kind: "collection", Label: "network WireGuard peer", GenDataSource: true},
	// wireless
	{Type: "wireless_device", Schema: "WirelessDevices", Collection: "wireless/devices", Kind: "collection", Label: "wireless device", GenDataSource: true, Descs: map[string]string{
		"path":         "Device path of the phy under /sys/devices, as `wifi config` writes it. With `macaddr` and `phy`, it binds the radio to its hardware: OpenWrt tries `path`, then `macaddr`, then `phy`, and a radio that none of them matches does not come up. Needs uapi 3.2.0 or later.",
		"macaddr":      "MAC address of the phy. Older `wifi detect` configurations bind a radio with it instead of `path`. Needs uapi 3.2.0 or later.",
		"phy":          "Name of the phy, as in board.json. `wifi config` writes it instead of `path` when the phy is not named phyN. Needs uapi 3.2.0 or later.",
		"radio":        "Index of the radio on a phy that has several, as `wifi config` writes it. Needs uapi 3.2.0 or later.",
		"cell_density": "Basic rates by coverage cell density: 0 disabled, 1 normal, 2 high, 3 very high. Defaults to 0. Needs uapi 3.2.0 or later.",
		"legacy_rates": "Whether the legacy 802.11b rates are allowed. Defaults to false. Needs uapi 3.2.0 or later.",
	}},
	{Type: "wireless_interface", Schema: "WirelessInterfaces", Collection: "wireless/interfaces", Kind: "collection", Label: "wireless interface", GenDataSource: true, Runtime: "wireless", Descs: map[string]string{
		"encryption":             "Encryption mode, such as `psk2`, `sae` or `owe`. A forced cipher follows a `+`, as LuCI writes it: `psk2+ccmp`.",
		"ieee80211r":             "Whether 802.11r fast BSS transition is on, for roaming between the access points of one mobility domain. Defaults to false. Needs uapi 3.2.0 or later.",
		"ft_over_ds":             "Whether 802.11r runs over the distribution system instead of over the air. Defaults to false. Needs uapi 3.2.0 or later.",
		"ft_psk_generate_local":  "Whether the 802.11r keys are generated locally instead of through the R0/R1 key holders. Unset means true when `encryption` is psk2 or psk-mixed, with or without a forced cipher, and false otherwise. 802.11r needs WPA2 or later, so plain psk gets no fast transition. Needs uapi 3.2.0 or later.",
		"mobility_domain":        "802.11r mobility domain, 4 hex digits. Unset, OpenWrt derives it from the SSID. Needs uapi 3.2.0 or later.",
		"nasid":                  "802.11r NAS identifier of this access point. Needs uapi 3.2.0 or later.",
		"reassociation_deadline": "802.11r reassociation deadline in time units of 1.024 ms, from 1000 to 65535. Unset, OpenWrt uses 20000. Needs uapi 3.2.0 or later.",
		"r0_key_lifetime":        "802.11r lifetime of the PMK-R0 in minutes. Unset, hostapd uses 10000. Needs uapi 3.2.0 or later.",
		"r1_key_holder":          "802.11r R1 key holder id, 12 hex digits. Unset, hostapd derives it from the BSSID. Needs uapi 3.2.0 or later.",
		"pmk_r1_push":            "Whether the PMK-R1 is pushed to the other key holders instead of being pulled by them. Defaults to false. Needs uapi 3.2.0 or later.",
		"r0kh":                   "802.11r R0 key holders, as `<MAC>,<NAS id>,<key>` entries. Write-only: never returned by the API, see `has_r0kh`. Removing it from the configuration keeps the entries on the router, as with `key`; set it to `[]` to remove them. Needs uapi 3.2.0 or later.",
		"r1kh":                   "802.11r R1 key holders, as `<MAC>,<R1KH id>,<key>` entries. Write-only: never returned by the API, see `has_r1kh`. Removing it from the configuration keeps the entries on the router, as with `key`; set it to `[]` to remove them. Needs uapi 3.2.0 or later.",
		"has_r0kh":               "Whether R0 key holders are configured (the entries are never returned).",
		"has_r1kh":               "Whether R1 key holders are configured (the entries are never returned).",
	}},
	// dhcp
	{Type: "dhcp_host", Schema: "DhcpHosts", Collection: "dhcp/hosts", Kind: "collection", Label: "dhcp host", GenDataSource: true, Descs: map[string]string{
		"name":      "Hostname dnsmasq answers for this reservation. A comma, a double quote, a `#` or a control character is refused, because the name goes into dnsmasq's config line.",
		"macs":      "MAC addresses for this reservation (the uci `list mac`). Takes precedence over the deprecated `mac` and `mac_aliases` when non-empty.",
		"tag":       "dnsmasq tags set on a request this reservation matches, for other options to match on (LuCI's Set Tag). This does not restrict the reservation: use `match_tag` for that. A response is always a list, including for a section stored as a space-separated scalar. A comma, a double quote, a `#` or a control character is refused.",
		"match_tag": "dnsmasq tags a request must carry for this reservation to apply (LuCI's Match Tag). Prefix a tag with `!` to negate it. Only a tag that dnsmasq sets from the request before it looks up hosts can match, such as one from a `config vendorclass`, `config userclass` or `config mac` rule. `known` and the DHCP range's own tag are set later, so `known` never matches and `!known` always does. A comma, a double quote, a `#`, a control character or a blank value is refused.",
	}, SchemaVersion: 1},
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
	{Type: "uhttpd_instance", Schema: "UhttpdInstances", Collection: "uhttpd/instances", Kind: "collection", Label: "uhttpd instance", GenDataSource: true, Descs: map[string]string{
		"key": "Path to the TLS private key file, not the key material itself.",
	}},
	{Type: "dropbear_instance", Schema: "DropbearInstances", Collection: "dropbear/instances", Kind: "collection", Label: "dropbear instance", GenDataSource: true, Descs: map[string]string{
		"interface": "Interface or IP address dropbear listens on.",
	}},
	{Type: "system_timeserver", Schema: "SystemTimeservers", Collection: "system/timeservers", Kind: "collection", Label: "system timeserver", GenDataSource: true},
	{Type: "system", Schema: "System", Collection: "system", Kind: "singleton", Label: "system settings", GenDataSource: true, Descs: map[string]string{
		"urandom_seed": "Path the entropy seed is saved to and restored from. A string as of uapi 2.5.0; it was previously modelled as a boolean by mistake.",
	}},
	{Type: "unbound_server", Schema: "UnboundServer", Collection: "unbound/server", Kind: "singleton", Label: "unbound server", GenDataSource: true},
	{Type: "unbound_srv", Schema: "UnboundSrv", Collection: "unbound/srv", Kind: "singleton", Label: "unbound srv options", GenDataSource: true},
	{Type: "unbound_ext", Schema: "UnboundExt", Collection: "unbound/ext", Kind: "singleton", Label: "unbound ext config", GenDataSource: true},
	{Type: "vnstat_config", Schema: "VnstatConfig", Collection: "vnstat/config", Kind: "singleton", Label: "vnstat config", GenDataSource: true, Descs: map[string]string{
		"interfaces": "Devices vnstat tracks, as the kernel names them (`br-lan`, `eth0`), not uci interface names. The only vnstat option any shipped code reads.",
	}},
	{Type: "lldpd_config", Schema: "LldpdConfig", Collection: "lldpd/config", Kind: "singleton", Label: "lldpd config", GenDataSource: true, Descs: map[string]string{
		"lldp_description": "System description advertised in LLDP frames. A string as of uapi 2.5.0; it was previously modelled as a boolean by mistake.",
	}},
	{Type: "prometheus_node_exporter_lua_config", Schema: "PrometheusNodeExporterLuaConfig", Collection: "prometheus_node_exporter_lua/config", Kind: "singleton", Label: "prometheus node_exporter config", GenDataSource: true},
	// mwan3 (added in uapi 2.0.0-rc3)
	{Type: "mwan3_interface", Schema: "Mwan3Interfaces", Collection: "mwan3/interfaces", Kind: "collection", Label: "mwan3 interface", GenDataSource: true},
	{Type: "mwan3_member", Schema: "Mwan3Members", Collection: "mwan3/members", Kind: "collection", Label: "mwan3 member", GenDataSource: true, Descs: map[string]string{
		"interface": "Name of an `uapi_mwan3_interface` section, not a network interface.",
	}},
	{Type: "mwan3_policy", Schema: "Mwan3Policies", Collection: "mwan3/policies", Kind: "collection", Label: "mwan3 policy", GenDataSource: true},
	{Type: "mwan3_rule", Schema: "Mwan3Rules", Collection: "mwan3/rules", Kind: "collection", Label: "mwan3 rule", GenDataSource: true},
	{Type: "mwan3_globals", Schema: "Mwan3Globals", Collection: "mwan3/globals", Kind: "singleton", Label: "mwan3 globals", GenDataSource: true},
	// usteer + openvpn (added in uapi 2.0.0-rc3; openvpn key/tls_auth/pkcs12 are write-only per the spec)
	{Type: "usteer_config", Schema: "UsteerConfig", Collection: "usteer/config", Kind: "singleton", Label: "usteer config", GenDataSource: true},
	{Type: "openvpn_instance", Schema: "OpenvpnInstances", Collection: "openvpn/instances", Kind: "collection", Label: "openvpn instance", GenDataSource: true, Descs: map[string]string{
		"key":      "Path to the private key PEM on the router, not the key material itself. Write-only: reads return `has_key`.",
		"tls_auth": "Path to the tls-auth/tls-crypt key file on the router, not the key material itself. Write-only: reads return `has_tls_auth`.",
		"pkcs12":   "Path to the PKCS#12 bundle on the router, not the bundle itself. Write-only: reads return `has_pkcs12`.",
	}},
}
