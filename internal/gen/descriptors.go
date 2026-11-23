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
	IntStr        []string // integer-or-string union fields (a number or a keyword on the wire), emitted as String; the body sends a numeric value as a number
	PatchUpdates  bool     // the generated Update sends a merge PATCH instead of a PUT: Terraform owns the modelled footprint, and unmodelled uci options that another tool wrote survive the update
	PostReadWarn  string   // hand-written func(obj map[string]any, diags *diag.Diagnostics) called by the generated resource Read after the model fill, for a refresh-time warning
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

// Shared by the firewall bools that fw4 parses: fw4 skips the section, or stops
// on firewall/defaults, for a value that it cannot parse, and uapi reads it null.
const (
	fwSkipBoolDesc     = " A stored value that firewall4 cannot parse, such as `YES` or a list, reads null, and firewall4 skips the section. The next apply then removes that value, and the attribute reads its default, unless the configuration sets it."
	fwDefaultsBoolDesc = " A stored value that firewall4 cannot parse, such as `YES` or a list, reads null, and firewall4 then applies no ruleset. uapi refuses every write of the defaults until one sets the attribute, so set it in the configuration."
)

// Shared by the zone policies, which carry x-uapi-clear-on-omit.
const zonePolicyDesc = " Unset, firewall4 applies the policy of `uapi_firewall_defaults`, and the attribute reads null. Removing it from the configuration removes the policy from the router, so the zone takes the policy of `uapi_firewall_defaults` again."

// Shared by the write-only secrets of network_interface.
const ifSecretDesc = " Removing it from the configuration keeps the stored value, and `\"\"` removes it. A switch to another proto removes it, unless the configuration sets it."

// Shared by unbound_srv and unbound_ext. unbound-uci-ext documents an unbound-checkconf run that
// nothing performs, so the grammar is only checked by unbound itself at the restart.
const unboundLineDesc = "Nothing checks their grammar before unbound restarts, and unbound stops on a line that it cannot parse. If unbound ran before the write, uapi 3.2.0 and later then restore the previous lines and answer `500 reload_failed_restored`."

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
	"probe_count":        "Probes per cycle per track_ip.",
	"download":           "Download shaping rate in kbit/s.",
	"upload":             "Upload shaping rate in kbit/s.",
	"ip_transparent":     "Bind to addresses that are not yet up (VIP / alias / floating addresses).",
	"interface_bind":     "Addresses unbound binds on (`addr` or `addr@port`). Pair with the main unbound `interface_auto = false` for exclusive binding.",
	"interface_outgoing": "Source addresses for upstream recursion (multi-WAN egress).",
	"srv_line":           "Verbatim lines inserted inside unbound's `server:` clause, one entry per line. " + unboundLineDesc,
	"ext_line":           "Verbatim lines rendered into unbound_ext.conf, outside the `server:` clause, one entry per line. Build whole `forward-zone:`, `view:` or `stub:` clauses by listing their lines in order. " + unboundLineDesc,
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
		"name":          "Zone name, referenced by rules and forwardings: a letter or an underscore, then letters, digits, dashes and underscores. firewall4 skips a zone whose name starts with a digit or a dash, so uapi refuses it.",
		"input":         "Policy for traffic from this zone to the router: `ACCEPT`, `REJECT` or `DROP`." + zonePolicyDesc,
		"output_policy": "Policy for traffic from the router to this zone: `ACCEPT`, `REJECT` or `DROP`." + zonePolicyDesc,
		"forward":       "Policy for traffic that the router forwards from this zone when no forwarding or rule matches: `ACCEPT`, `REJECT` or `DROP`." + zonePolicyDesc,
		"masq":          "Whether firewall4 masquerades IPv4 traffic that leaves through this zone. Unset means false." + fwSkipBoolDesc,
		"mtu_fix":       "Whether firewall4 clamps the TCP MSS of traffic that leaves through this zone to the path MTU. Unset means false." + fwSkipBoolDesc,
	}},
	{Type: "firewall_rule", Schema: "FirewallRules", Collection: "firewall/rules", Kind: "collection", Label: "firewall rule", GenDataSource: true, Nested: matchFields(false), Descs: map[string]string{
		"enabled": "Whether the rule is active. Unset means true." + fwSkipBoolDesc,
		"target":  "What to do with matched traffic: `ACCEPT`, `REJECT`, `DROP`, `NOTRACK`, `MARK` or `DSCP`. A read also gives `HELPER`, which LuCI writes for a conntrack helper rule. uapi does not model the helper and refuses `HELPER` in a create and in a PUT, because a PUT removes the `set_helper` option and firewall4 skips a HELPER rule without it. The provider updates a rule with a PUT, so it can read such a rule but not update it.",
	}},
	{Type: "firewall_redirect", Schema: "FirewallRedirects", Collection: "firewall/redirects", Kind: "collection", Label: "firewall redirect", GenDataSource: true, Nested: matchFields(true), SchemaVersion: 1, Descs: map[string]string{
		"enabled":    "Whether the redirect is active. Unset means true." + fwSkipBoolDesc,
		"reflection": "Whether firewall4 adds NAT reflection rules, so that internal hosts reach the redirect through the external address. Unset, firewall4 adds them. A stored value that firewall4 cannot parse, such as `YES`, reads null too, and firewall4 then skips the section.",
	}},
	{Type: "firewall_forwarding", Schema: "FirewallForwardings", Collection: "firewall/forwardings", Kind: "collection", Label: "firewall forwarding", GenDataSource: true, Descs: map[string]string{
		"enabled": "Whether the forwarding is active. Unset means true." + fwSkipBoolDesc,
	}},
	{Type: "firewall_nat", Schema: "FirewallNat", Collection: "firewall/nat", Kind: "collection", Label: "firewall NAT rule", GenDataSource: true, Nested: natMatchFields(), Descs: map[string]string{
		"target":  "What to do with matched traffic: `SNAT` rewrites the source to `snat_ip`/`snat_port`, `MASQUERADE` rewrites it to the outbound interface address, `ACCEPT` exempts it from source NAT. Defaults to `MASQUERADE`. A read also gives `NPT`, which the firewall4 of OpenWrt main takes. uapi refuses a change of `target` to `NPT`, and from uapi 3.5.0 an update that keeps a stored `NPT` passes.",
		"snat_ip": "IPv4 address to rewrite the source to. Requires target `SNAT`. On a stored `NPT` rule, the prefix that firewall4 maps `match.src_ip` to.",
		"name":    "Human-readable label for this NAT rule.",
		"enabled": "Whether the NAT rule is active. Unset means true." + fwSkipBoolDesc,
	}},
	{Type: "firewall_defaults", Schema: "FirewallDefaults", Collection: "firewall/defaults", Kind: "singleton", Label: "firewall defaults", GenDataSource: true, Descs: map[string]string{
		"input":              "Default policy for traffic to the router, which a zone without its own `input` takes: `ACCEPT`, `REJECT` or `DROP`.",
		"output_policy":      "Default policy for traffic from the router, which a zone without its own `output_policy` takes: `ACCEPT`, `REJECT` or `DROP`.",
		"forward":            "Default policy for forwarded traffic, which a zone without its own `forward` takes: `ACCEPT`, `REJECT` or `DROP`.",
		"syn_flood":          "Whether firewall4 limits incoming TCP SYN packets to `synflood_rate`. A read gives what firewall4 applies: LuCI writes `synflood_protect`, which firewall4 reads first. uapi writes true as `synflood_protect` too. Unset means false." + fwDefaultsBoolDesc,
		"synflood_rate":      "SYN packets that `syn_flood` lets through, as a rate with its whole unit, such as `25/second` or `100/minute`. A read gives the rate as firewall4 applies it, so a stored `25` or `25/s` reads `25/second`. uapi refuses a bare number such as `25` and a unit prefix such as `25/m`, so write the whole unit. Unset, firewall4 applies 25/second. A string since provider 4.0.",
		"drop_invalid":       "Whether firewall4 drops packets that conntrack marks invalid. Unset means false." + fwDefaultsBoolDesc,
		"tcp_syncookies":     "Kernel TCP SYN cookies. Unset reads false." + fwDefaultsBoolDesc,
		"flow_offloading":    "Whether firewall4 offloads established flows to the software flow table. Unset means false." + fwDefaultsBoolDesc,
		"flow_offloading_hw": "Whether firewall4 offloads established flows to the hardware, when the driver can. Needs `flow_offloading`. Unset means false." + fwDefaultsBoolDesc,
	}},
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
			"proto":          "Interface protocol: `static`, `dhcp`, `dhcpv6`, `pppoe`, `ppp`, `wwan`, `wireguard`, `openvpn` or `none`. A proto needs its handler package on the router, such as `openvpn-openssl` for `openvpn`. netifd loads a handler that is installed later only after a reboot or `/etc/init.d/network restart`, and until then `runtime.effective_proto` reads `none`. When it changes, the plan sets to null each attribute that the new proto does not take. uapi also reads a proto that it does not model, such as a `6in4` tunnel from LuCI. An update of such an interface is a PATCH of the changed attributes, which keeps the options of its handler. uapi refuses a switch to such a proto.",
			"broadcast":      "IPv4 broadcast address. On proto `dhcp`, the same uci option is a flag: `broadcast_flag`.",
			"broadcast_flag": "Whether udhcpc asks the DHCP server to broadcast its replies (`-B`), as LuCI's \"Use broadcast flag\". uci stores it as `broadcast`, which is the broadcast address on proto `static`. Unset means false. Needs uapi 3.2.0 or later.",
			"clientid":       "Client identifier: DHCP option 61 on proto `dhcp`, a DUID on proto `dhcpv6`.",
			"reqprefix":      "Prefix delegation request: `auto`, `no`, a prefix length such as `56`, or a prefix with its length such as `2001:db8::/56`. From uapi 3.5.0, a prefix length or a prefix can carry a `:IAID` suffix of up to 8 hex digits, as odhcp6c reads it.",
			"sourcefilter":   "Whether the default route from upstream is source-specific. Unset means true. A routed `ip6prefix` needs false: otherwise traffic from its downstream subnets matches no default route. Needs uapi 3.2.0 or later.",
			"listen_port":    "UDP port that WireGuard listens on. Null or 0 lets the kernel pick one. A read gives the number that the wireguard handler gives wg: a service name that uci holds, such as `openvpn`, reads null, and wg then listens on a port that the kernel picks. A stored number outside 0 to 65535 reads -1: wg refuses the whole interface, and uapi refuses every write of the interface until one sets a port.",
			"ip6prefix":      "IPv6 prefixes routed to this device, for delegation to clients. uapi stores one prefix as a uci option and several as a uci list. A list since provider 3.2.0, which needs uapi 3.2.0 or later.",
			"mtu":            "MTU that netifd applies to the devices of this interface, from 68. A device with its own `config device` section keeps its own MTU: set it on `uapi_network_device`. On proto `ppp` and `pppoe`, pppd takes 128 to 16384. On proto `wireguard`, the handler applies it to the tunnel.",
			"disabled":       "Whether netifd ignores this interface. A disabled interface has no ubus object, and netifd installs none of its addresses and routes. A stored `1` or `true` reads true.",
			"metric":         "Metric of the routes that this interface installs without a metric of their own. Applies to every proto.",
			"defaultroute":   "Whether to install the default route that the protocol provides. Unset means true. Applies to every proto. On proto `openvpn`, openvpn-hotplug provides one each time openvpn knows a route gateway, which a `topology subnet` server always pushes, also without `redirect-gateway`. netifd then replaces the default route of wan, and when the tunnel goes down, the main table has no IPv4 default route until wan restarts. Set it to false on an openvpn client that must not carry the default route.",
			"delegate":       "Whether to hand the IPv6 prefixes of this interface on to downstream interfaces. Unset means true. Applies to every proto.",
			"ip4table":       "Routing table for the IPv4 routes that this interface installs: a number, or a name from `/etc/iproute2/rt_tables`. Applies to every proto.",
			"ip6table":       "Routing table for the IPv6 routes that this interface installs: a number, or a name from `/etc/iproute2/rt_tables`. Applies to every proto.",
			"ip6assign":      "Length of the prefix that this interface takes from a delegated or ULA prefix, such as `64`. Applies to every proto.",
			"ip6hint":        "Hexadecimal subnet id that selects the part of a delegated or ULA prefix that this interface gets, such as `1` or `0x1f`. netifd uses it only when `ip6assign` is set. Applies to every proto.",
			"ip6class":       "Prefix classes that this interface can take its subnet from: `local` for the ULA prefix, or the name of the interface that received the prefix. Unset means any. Needs uapi 3.2.0 or later.",
			"ip6weight":      "Assignment preference: when interfaces share a prefix, the interfaces with a higher weight get their subnet first. Needs uapi 3.2.0 or later.",
			// proto openvpn (uapi 4.0) and the login of pppoe, ppp, wwan and openvpn.
			"auth":              "HMAC digest, such as `SHA256` (`--auth`).",
			"ca":                "Absolute path of the CA certificate PEM. The handler leaves the option out when the file does not exist at ifup, so openvpn then runs as if it were unset. A write does not check that the file exists.",
			"cert":              "Absolute path of the local certificate PEM. The handler leaves the option out when the file does not exist at ifup, so openvpn then runs as if it were unset. A write does not check that the file exists.",
			"cert_password":     "Passphrase of the private key, which the handler writes to a file that it gives openvpn as `--askpass`. Write-only: a read gives `has_cert_password`." + ifSecretDesc,
			"cipher":            "Data cipher for a peer that negotiates none, such as `AES-256-CBC` (`--cipher`).",
			"client":            "Run in client mode (`--client`), which takes the options that the server pushes. Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"client_to_client":  "Server mode: route traffic between clients inside openvpn (`--client-to-client`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"config":            "Absolute path of an openvpn config file, which openvpn reads beside these options (`--config`). LuCI's raw config tab stores `/etc/openvpn/<id>/<id>_config.cfg`. The handler runs openvpn in the directory of this file, so a relative path in it resolves there. The handler leaves the option out when the file does not exist at ifup, so openvpn then runs as if it were unset. A write does not check that the file exists.",
			"crl_verify":        "Absolute path of the certificate revocation list PEM. The handler leaves the option out when the file does not exist at ifup, so openvpn then runs as if it were unset. A write does not check that the file exists.",
			"data_ciphers":      "Ciphers that openvpn negotiates, as one entry of names joined by colons, such as `AES-256-GCM:CHACHA20-POLY1305`. It replaces `ncp_ciphers`. Set one entry: the handler gives openvpn each entry as a `--data-ciphers` of its own, openvpn keeps only the last, and uapi refuses several entries.",
			"dev":               "tun or tap device name, such as `tun0`. openvpn needs a device, from this field or from `config`.",
			"dev_type":          "`tun` or `tap`. Unset, the handler passes `--dev-type tun`. openvpn also takes `null`, a device that carries no traffic, which a read gives and uapi refuses in a write.",
			"dh":                "Absolute path of the Diffie-Hellman parameters PEM, for server mode. The handler leaves the option out when the file does not exist at ifup, so openvpn then runs as if it were unset. A write does not check that the file exists.",
			"duplicate_cn":      "Server mode: let several clients connect with one common name (`--duplicate-cn`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"float":             "Let the peer change its address or port (`--float`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"group":             "Group that openvpn drops its privileges to.",
			"has_cert_password": "Whether uci holds `cert_password` (the value is never returned).",
			"has_key":           "Whether uci holds `key` (the path is never returned).",
			"has_password":      "Whether uci holds `password` (the value is never returned).",
			"has_pkcs12":        "Whether uci holds `pkcs12` (the path is never returned).",
			"has_tls_auth":      "Whether uci holds `tls_auth` (the path is never returned).",
			"ifconfig":          "Tunnel addresses: `local remote` on a point-to-point tun, `address netmask` otherwise.",
			"ifconfig_pool":     "Server-mode address pool: `start end [netmask]`.",
			"keepalive":         "`interval timeout` pair, such as `10 60`.",
			"key":               "Absolute path of the private key PEM on the router, not the key material itself. The handler leaves the option out when the file does not exist at ifup. Write-only: a read gives `has_key`." + ifSecretDesc,
			"local":             "Local IP address or host name to bind to (`--local`), as LuCI's form takes it.",
			"log":               "Absolute path of a log file, which openvpn truncates at each start (`--log`).",
			"log_append":        "Absolute path of a log file that openvpn appends to (`--log-append`).",
			"lport":             "Local port (`--lport`).",
			"mute":              "Log at most this many messages of one category in a row (`--mute`).",
			"nobind":            "Bind to no local address and port (`--nobind`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"ovpnproto":         "Transport protocol, openvpn's `--proto`, such as `udp` or `tcp-client`. uci names it `ovpnproto` because `proto` is the netifd protocol. It is the `proto` of the 3.x `uapi_openvpn_instance`. Unset, openvpn uses udp.",
			"password":          "Password that the interface logs in with, beside `username`. On proto `pppoe` and `ppp`, a password without `username` does nothing, because ppp.sh then gives pppd neither. Write-only: a read gives `has_password`." + ifSecretDesc,
			"persist_key":       "Keep the keys across a restart (`--persist-key`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"persist_tun":       "Keep the tun or tap device across a restart (`--persist-tun`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"ping":              "Seconds between pings when no other traffic passes (`--ping`).",
			"ping_restart":      "Seconds without traffic from the peer before openvpn restarts (`--ping-restart`).",
			"pkcs12":            "Absolute path of the PKCS#12 bundle on the router, not the bundle itself. The handler leaves the option out when the file does not exist at ifup. Write-only: a read gives `has_pkcs12`." + ifSecretDesc,
			"port":              "Local and remote port (`--port`).",
			"push":              "Server-mode directives pushed to clients, such as `route 10.0.0.0 255.255.255.0`. netifd splits an option that is not a uci list into its words, and the handler gives openvpn each as an entry of its own, so such an option reads as its words.",
			"remote":            "Servers that a client connects to, each `host [port [proto]]`, tried in order. netifd splits an option that is not a uci list into its words, and the handler gives openvpn each as an entry of its own, so such an option reads as its words.",
			"remote_cert_tls":   "Require that the peer certificate carries the key usage of a client or of a server.",
			"route":             "Routes of the tunnel, each `network [netmask [gateway [metric]]]`, which openvpn-hotplug hands to netifd. netifd splits an option that is not a uci list into its words, and the handler gives openvpn each as an entry of its own, so such an option reads as its words.",
			"route_gateway":     "Gateway of the `route` entries that name none.",
			"rport":             "Remote port (`--rport`).",
			"server":            "Server-mode subnet, such as `10.8.0.0 255.255.255.0`.",
			"server_bridge":     "Bridged server mode: `gateway netmask pool-start pool-end`.",
			"tls_auth":          "Absolute path of the tls-auth key file on the router, not the key material itself. Write-only: a read gives `has_tls_auth`." + ifSecretDesc,
			"tls_client":        "Take the TLS client role (`--tls-client`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"tls_crypt":         "Absolute path of the tls-crypt key file. The handler leaves the option out when the file does not exist at ifup, so openvpn then runs as if it were unset. A write does not check that the file exists.",
			"tls_server":        "Take the TLS server role (`--tls-server`). Only a stored `1` or `true` reads true: netifd drops another value, and the handler then passes no flag.",
			"topology":          "Server-mode topology.",
			"user":              "User that openvpn drops its privileges to.",
			"username":          "User name that the interface logs in with. ppp.sh gives it and `password` to pppd, and gives neither when it is unset. wwan hands both to the handler of the modem that it finds, such as qmi or mbim. The openvpn handler writes both to a file that it gives openvpn as `--auth-user-pass`. Removing it from the configuration removes it from the router, and so does a switch to another proto.",
			"verb":              "Log verbosity. uapi takes 0 to 11.",
			"verify_x509_name":  "Accept only a peer certificate of this name: `name [type]`.",
		}},
	{Type: "network_device", Schema: "NetworkDevices", Collection: "network/devices", Kind: "collection", Label: "network device", GenDataSource: true, Descs: map[string]string{
		"name": "Device name as netifd and the kernel see it (`br-lan`, `eth0`).",
	}},
	{Type: "network_route", Schema: "NetworkRoutes", Collection: "network/routes", Kind: "collection", Label: "network route", GenDataSource: true, Descs: map[string]string{
		"target":    "Destination IPv4 address or CIDR for this route.",
		"interface": "Parent network interface name.",
	}},
	{Type: "network_rule", Schema: "NetworkRules", Collection: "network/rules", Kind: "collection", Label: "network rule", GenDataSource: true, IntStr: []string{"ipproto"}, Descs: map[string]string{
		"suppress_prefixlength": "Ignore a route that the lookup of this rule finds when its prefix is this long or shorter, and go on to the next rule: 0 ignores only a default route. A rule that looks up `main` with 0, placed ahead of a rule that sends a source subnet to a VPN table, keeps the LAN reachable in one section, which otherwise needs one rule per destination. 0 to 32. Needs uapi 3.3.0 or later.",
		"tos":                   "Match this TOS value: 0, 4, 8, 12, 16, 20, 24 or 28, as netifd reads an integer. Needs uapi 3.3.0 or later.",
		"uidrange":              "Match traffic of local sockets owned by this uid, or by a range such as `1000-1005`, each from 0 to 4294967294. Needs uapi 3.3.0 or later.",
		"ipproto":               "Match this IP protocol, as an integer from 1 to 255, which LuCI writes. A read also gives another spelling that uci holds and netifd takes, such as `tcp` or `06`. An update that keeps such a value passes, but uapi refuses a change to one, so write the number. A decimal reads as an integer.",
		"sport":                 "Match this source port, or a range such as `1000-2000`, each from 1 to 65534. Needs uapi 3.3.0 or later.",
		"dport":                 "Match this destination port, or a range such as `1000-2000`, each from 1 to 65534. Needs uapi 3.3.0 or later.",
	}},
	{Type: "network_bridge_vlan", Schema: "NetworkBridgeVlans", Collection: "network/bridge_vlans", Kind: "collection", Label: "network bridge VLAN", GenDataSource: true},
	{Type: "network_wireguard_peer", Schema: "NetworkWireguardPeers", Collection: "network/wireguard_peers", Kind: "collection", Label: "network WireGuard peer", GenDataSource: true, Runtime: "wireguard", PostReadWarn: "warnWireguardPeerNotInKernel", Descs: map[string]string{
		"allowed_ips":          "Routes that go through the peer. A peer without allowed IPs is valid: wg sets it with none. Leaving the field out or setting it to `[]` removes the list.",
		"endpoint_port":        "UDP port of the endpoint. A service name that uci holds, such as `https`, reads as its port from /etc/services, as the proto handler resolves it. A name that does not resolve reads 0, and the handler then leaves the endpoint out. uapi refuses every write of the peer while the port reads 0, so set a port. A peer with an `endpoint_host` and no port reads null, and the handler gives wg port 51820 for it.",
		"persistent_keepalive": "Seconds between keepalive packets. 0 or unset turns them off. A stored value that the proto handler cannot read, such as `25x`, reads -1, and the handler then leaves the keepalive out. uapi refuses every write of the peer while the keepalive reads -1, so set a value.",
	}},
	// wireless
	{Type: "wireless_device", Schema: "WirelessDevices", Collection: "wireless/devices", Kind: "collection", Label: "wireless device", GenDataSource: true, IntStr: []string{"channel"}, PatchUpdates: true, Descs: map[string]string{
		"channel":      "Channel number (0-233, where 6 GHz uses up to 233) or `auto`. A numeric value may be written as a number or as its decimal text; any other string is refused by uapi.",
		"path":         "Device path of the phy under /sys/devices, as `wifi config` writes it. With `macaddr` and `phy`, it binds the radio to its hardware: OpenWrt tries `path`, then `macaddr`, then `phy`, and a radio that none of them matches does not come up. Needs uapi 3.2.0 or later.",
		"macaddr":      "MAC address of the phy. Older `wifi detect` configurations bind a radio with it instead of `path`. Needs uapi 3.2.0 or later.",
		"phy":          "Name of the phy, as in board.json. `wifi config` writes it instead of `path` when the phy is not named phyN. Needs uapi 3.2.0 or later.",
		"radio":        "Index of the radio on a phy that has several, as `wifi config` writes it. Needs uapi 3.2.0 or later.",
		"cell_density": "Basic rates by coverage cell density: 0 disabled, 1 normal, 2 high, 3 very high. Defaults to 0. Needs uapi 3.2.0 or later.",
		"legacy_rates": "Whether the legacy 802.11b rates are allowed. Defaults to false. Needs uapi 3.2.0 or later.",
	}},
	{Type: "wireless_interface", Schema: "WirelessInterfaces", Collection: "wireless/interfaces", Kind: "collection", Label: "wireless interface", GenDataSource: true, Runtime: "wireless", PatchUpdates: true, SchemaVersion: 1, Descs: map[string]string{
		"device":                 "Radios of this SSID, as `uapi_wireless_device` ids. A read gives a list, also for one radio that uci holds as a string. A list since provider 4.0.",
		"network":                "Network interfaces that this SSID joins. A read gives a list, also for one network that uci holds as a string. A list since provider 4.0.",
		"mode":                   "Operating mode: `ap`, `sta`, `adhoc`, `monitor` or `mesh`. A section without a mode reads null, and wifi-scripts sets up nothing for it. uapi stores no mode when the configuration sets none, so set `ap` for an access point. A read also gives `wds` or `ahdemo`, which wifi-scripts skips and uapi refuses in a write.",
		"ssid":                   "Network name that the access point broadcasts. A list that uci holds reads as its JSON text, such as `[ \"home\", \"net\" ]`, which hostapd broadcasts.",
		"encryption":             "Encryption mode, such as `psk2`, `sae` or `owe`. A forced cipher follows a `+`, as LuCI writes it: `psk2+ccmp`. From uapi 3.5.0, a write of a WEP value (`wep`, `wep-open`, `wep-shared`) answers 422 unless the router sets WEP up, which needs the shell wifi-scripts. A stock image builds the ucode wifi-scripts, which give the daemon no WEP key, so the SSID would run without encryption.",
		"ieee80211r":             "Whether 802.11r fast BSS transition is on, for roaming between the access points of one mobility domain. Defaults to false. Needs uapi 3.2.0 or later.",
		"ft_over_ds":             "Whether 802.11r runs over the distribution system instead of over the air. Defaults to false. Needs uapi 3.2.0 or later.",
		"ft_psk_generate_local":  "Whether the 802.11r keys are generated locally instead of through the R0/R1 key holders. Unset means true when `encryption` is psk2 or psk-mixed, with or without a forced cipher, and false otherwise. 802.11r needs WPA2 or later, so plain psk gets no fast transition. Needs uapi 3.2.0 or later.",
		"mobility_domain":        "802.11r mobility domain, 4 hex digits. Unset, OpenWrt derives it from the SSID. Needs uapi 3.2.0 or later.",
		"nasid":                  "802.11r NAS identifier of this access point. Needs uapi 3.2.0 or later.",
		"reassociation_deadline": "802.11r reassociation deadline in time units of 1.024 ms, from 1000 to 65535. Unset, OpenWrt uses 20000. Needs uapi 3.2.0 or later.",
		"r0_key_lifetime":        "802.11r lifetime of the PMK-R0 in minutes. Unset, hostapd uses 14 days (20160 minutes). 0 turns the timeout off. Needs uapi 3.2.0 or later.",
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
		"macs":      "MAC addresses for this reservation, one per entry (the uci `list mac`). From uapi 3.2.0, an option or a list entry that holds several MACs separated by spaces reads as one entry per MAC, as dnsmasq gets them. uapi refuses an entry with a space, so list each MAC as its own entry. From uapi 3.3.0, an octet can be `*`, which dnsmasq matches against any value, as in `00:11:22:33:44:*`; odhcpd skips the whole host when it cannot parse one of its MACs, as it cannot parse one with `*`, so it then gives this host no lease, not even for its duid.",
		"tag":       "dnsmasq tags set on a request this reservation matches, for other options to match on (LuCI's Set Tag). This does not restrict the reservation: use `match_tag` for that. A response is always a list, including for a section stored as a space-separated scalar. From uapi 3.3.0, a list entry that holds several words reads as one tag per word, as dnsmasq.init splits them. A comma, a double quote, a `#` or a control character is refused.",
		"match_tag": "dnsmasq tags a request must carry for this reservation to apply (LuCI's Match Tag). Prefix a tag with `!` to negate it. Only a tag that dnsmasq sets from the request before it looks up hosts can match, such as one from a `config vendorclass`, `config userclass` or `config mac` rule. `known` and the DHCP range's own tag are set later, so `known` never matches and `!known` always does. A comma, a double quote, a `#`, a control character or a blank value is refused.",
	}, SchemaVersion: 1},
	{Type: "dhcp_server", Schema: "DhcpServers", Collection: "dhcp/servers", Kind: "collection", Label: "dhcp server", GenDataSource: true, Descs: map[string]string{
		"dhcpv4":      "DHCPv4 service on this interface: `server`, `disabled`, `relay` or `hybrid`. dnsmasq serves on every value but `disabled`, and takes an unset value as `disabled`, so set `server` to serve. odhcpd has no DHCPv4 relay and serves on every value but `disabled` too, but only when `dhcp/odhcpd` sets `maindhcp`, and it takes an unset option as `disabled`. LuCI offers only `server` and `disabled`, and its DHCP setup writes `server`. odhcpd.defaults writes it into the stock `lan`. Reads null when unset. Needs uapi 3.3.0 or later.",
		"ra_flags":    "Flags of the router advertisements: `managed-config` (clients ask for a DHCPv6 address), `other-config`, `home-agent`, or `none`, which LuCI writes when no flag is selected. The stock `lan` carries `managed-config` and `other-config`, and a PUT that left the field out removed them, so clients that follow the managed flag stopped asking DHCPv6. Unset reads null, which odhcpd takes as `other-config` only, and `none` sets no flag. Needs uapi 3.3.0 or later.",
		"ra_slaac":    "Whether the router advertisements offer SLAAC addressing. odhcpd.defaults writes it into the stock `lan`, and a PUT that left the field out removed it. Reads true when unset, as odhcpd takes unset. odhcpd reads only `1`, `true`, `0` and `false`, and takes true for any other stored value. Needs uapi 3.3.0 or later.",
		"dhcp_option": "DHCP options that dnsmasq sends, one per entry, such as `6,192.168.1.1`. dnsmasq.init passes each list entry whole, and dnsmasq reads it as one option. From uapi 3.2.0, an option that holds several words reads as one entry per word, as dnsmasq.init splits it. uapi 3.2.0 also refuses an entry in which an address is followed by whitespace and another option, such as `3,10.0.0.1 6,10.0.0.1`. A space in a text option, such as `15,my domain`, is valid.",
	}},
	{Type: "dhcp_dnsmasq", Schema: "DhcpDnsmasq", Collection: "dhcp/dnsmasq", Kind: "singleton", Label: "dnsmasq settings", GenDataSource: true, Descs: map[string]string{
		"server":  "Upstream DNS servers, one per entry, each a dnsmasq `server`, such as `10.128.2.1` or `/example.com/192.0.2.1`. From uapi 3.3.0, a read gives what dnsmasq gets: dnsmasq.init passes only the entries of a list, so a stored scalar `option server`, which dnsmasq ignores, reads as null, and a write that sends a list stores the list in its place.",
		"address": "Fixed DNS answers, one per entry, each a dnsmasq `address`, such as `/example.lan/192.0.2.1`. From uapi 3.3.0, a read gives what dnsmasq gets: dnsmasq.init passes only the entries of a list, so a stored scalar `option address`, which dnsmasq ignores, reads as null, and a write that sends a list stores the list in its place.",
	}},
	{Type: "dhcp_odhcpd", Schema: "DhcpOdhcpd", Collection: "dhcp/odhcpd", Kind: "singleton", Label: "odhcpd settings", GenDataSource: true, Descs: map[string]string{
		"maindhcp": "Whether odhcpd serves DHCPv4 in place of dnsmasq. dnsmasq.init reads only the section named `odhcpd`: dnsmasq stops serving DHCPv4 for a value above 0 there, while odhcpd is installed. From uapi 3.3.0, a read gives what dnsmasq.init reads: true only for `1` in that section. A write of true stores `1` and of false `0`, so a write that sends the field makes both daemons agree. A stored `true` or `2` reads false: odhcpd keeps its own value, which a reload does not reset. null removes the option, and odhcpd then keeps its value until it restarts.",
	}},
	// snmpd
	{Type: "snmpd_access", Schema: "SnmpdAccesses", Collection: "snmpd/accesses", Kind: "collection", Label: "snmpd access", GenDataSource: true, Descs: map[string]string{
		"group":   "snmpd group that this access binds to, as `uapi_snmpd_group.group`. The group must exist.",
		"context": "SNMPv3 context. `none` is the empty context, which is the usual value: snmpd.init writes it as `\"\"`.",
		"version": "Security model: `any`, `v1`, `v2c` or `usm`.",
		"level":   "Security level: `noauth`, `auth` or `priv`.",
		"prefix":  "How the context matches: `exact`, or `prefix` for every context that starts with it.",
		"read":    "View that the group can read, such as `all`, or `none`.",
		"write":   "View that the group can write, or `none`.",
		"notify":  "View that the group gets notifications for, or `none`.",
	}},
	{Type: "snmpd_agent", Schema: "SnmpdAgents", Collection: "snmpd/agents", Kind: "collection", Label: "snmpd agent", GenDataSource: true},
	{Type: "snmpd_com2sec", Schema: "SnmpdCom2secs", Collection: "snmpd/com2secs", Kind: "collection", Label: "snmpd com2sec", GenDataSource: true},
	{Type: "snmpd_group", Schema: "SnmpdGroups", Collection: "snmpd/groups", Kind: "collection", Label: "snmpd group", GenDataSource: true},
	{Type: "snmpd_system", Schema: "SnmpdSystem", Collection: "snmpd/system", Kind: "singleton", Label: "snmpd system", GenDataSource: true},
	// sqm / uhttpd / dropbear / system / vnstat / unbound / lldpd / prometheus
	{Type: "sqm_queue", Schema: "SqmQueues", Collection: "sqm/queues", Kind: "collection", Label: "sqm queue", GenDataSource: true, Descs: map[string]string{
		"interface": "Network device that sqm-scripts shapes, such as `eth1` or `pppoe-wan`: its hotplug starts the queue when this device comes up. The name of a network interface, such as `wan`, matches no device unless one has that name. The `runtime.l3_device` of `data.uapi_network_interface` gives the device of an interface. 1 to 15 characters without `/`, `:` or whitespace. uapi does not check that the device exists, because a device such as `pppoe-wan` exists only while its interface is up. One queue per device: uapi refuses a second one, which sqm-scripts would never run.",
	}},
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
		"interface":        "Interfaces on which lldpd runs: a uci interface name, which OpenWrt resolves to its device, a device name, or a pattern such as `eth*` or `!eth1`. Unset, lldpd runs on every interface. From uapi 3.2.0, an option or a list entry that holds several names separated by spaces reads as one entry per name, as lldpd.init splits it. uapi refuses an entry with a space, a quote, a backslash or a `#`.",
	}},
	{Type: "prometheus_node_exporter_lua_config", Schema: "PrometheusNodeExporterLuaConfig", Collection: "prometheus_node_exporter_lua/config", Kind: "singleton", Label: "prometheus node_exporter config", GenDataSource: true},
	// mwan3 (added in uapi 2.0.0-rc3)
	{Type: "mwan3_interface", Schema: "Mwan3Interfaces", Collection: "mwan3/interfaces", Kind: "collection", Label: "mwan3 interface", GenDataSource: true, Descs: map[string]string{
		"family": "Address family that this interface tracks: `ipv4` or `ipv6`.",
	}},
	{Type: "mwan3_member", Schema: "Mwan3Members", Collection: "mwan3/members", Kind: "collection", Label: "mwan3 member", GenDataSource: true, Descs: map[string]string{
		"interface": "Name of an `uapi_mwan3_interface` section, not a network interface.",
	}},
	{Type: "mwan3_policy", Schema: "Mwan3Policies", Collection: "mwan3/policies", Kind: "collection", Label: "mwan3 policy", GenDataSource: true},
	{Type: "mwan3_rule", Schema: "Mwan3Rules", Collection: "mwan3/rules", Kind: "collection", Label: "mwan3 rule", GenDataSource: true, Descs: map[string]string{
		"family": "Address family of the traffic that this rule matches: `ipv4`, `ipv6` or `any`. Unset or `any`, mwan3 applies the rule to both. A write of `any`, which LuCI writes, needs uapi 3.5.0 or later.",
		"proto":  "IP protocol as iptables takes it: a number from 0 to 255, or a name that iptables resolves, such as `tcp`, `udp`, `icmp`, `esp` or `all`. uapi refuses another value, because iptables refuses the rules of the whole family for it. Before uapi 3.5.0, a write takes only `tcp`, `udp`, `icmp` and `all`.",
	}},
	{Type: "mwan3_globals", Schema: "Mwan3Globals", Collection: "mwan3/globals", Kind: "singleton", Label: "mwan3 globals", GenDataSource: true, Descs: map[string]string{
		"loglevel": "Level of the iptables LOG rules of mwan3: `emerg`, `alert`, `crit`, `error`, `warning`, `notice`, `info` or `debug`, as LuCI offers. iptables refuses `err` and `warn`, and mwan3 then loads none of its rules, so uapi refuses them. A read also gives another level that iptables takes, such as `panic` or `3`, and an update that keeps it passes.",
	}},
	// usteer (added in uapi 2.0.0-rc3)
	{Type: "usteer_config", Schema: "UsteerConfig", Collection: "usteer/config", Kind: "singleton", Label: "usteer config", GenDataSource: true},
}
