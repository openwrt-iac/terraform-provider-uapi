# Changelog

All notable changes are documented here. The provider mirrors uapi: provider
`x.y.*` covers the curated surface of uapi `x.y.*` (patch is the provider's own
line). Format follows Keep a Changelog.

## [Unreleased]

Targets the uapi 1.2.x curated surface. Purely additive over 1.1.

### Added
- ETag / If-Match optimistic concurrency: every resource carries a computed
  `etag`; updates and deletes send `If-Match`, and a stale write (out-of-band
  change since the last refresh) fails with a clear "changed outside Terraform"
  error (HTTP 412) instead of clobbering.
- `uapi_authorized_key` resource and data source (root SSH `authorized_keys`).
- `uapi_system_password` resource with a true write-only `password_wo` attribute
  (never stored in state; bump `password_wo_version` to re-apply).
- `uapi_dhcp_leases6` data source (active IPv6 / odhcpd leases).
- Computed `runtime` block (live ubus state) on the `uapi_network_interface` and
  `uapi_wireless_interface` data sources.
- `network_interface`: dhcp/dhcpv6 client options (`peerdns`, `defaultroute`,
  `metric`, `hostname`, `clientid`, `reqprefix`, `reqaddress`, `ip6hint`,
  `ip6ifaceid`, `delegate`) and `ipaddrs`.
- `firewall_redirect`: NAT loopback (`reflection`, `reflection_src`, `reflection_zone`).
- `dhcp_host`: `duid`, `hostid`, `mac_aliases`, `broadcast`, `instance`
  (`mac` is now optional, since uapi accepts mac OR duid).
- `unbound_server`: `manual_conf`, `extended_stats`, `interface_auto`,
  `localservice`, `hide_binddata`, `rebind_protection`, `num_threads`,
  `ttl_min`, `domain`, `domain_type`.
- `terraform-plugin-testing` acceptance suite run against an in-process fake
  uapi (no router needed), wired into CI; `tflog` request/response tracing in
  the client (never logs secrets).

## [1.1.0]

### Added
- Full uapi 1.1 curated surface: 16 CRUD resources (network routes/rules/
  bridge_vlans/wireguard_peers, firewall forwardings, dhcp servers, snmpd
  accesses/agents/com2secs/groups, sqm queues, system timeservers, uhttpd
  certs/instances, vnstat interfaces), 8 singletons, and `packages/*`
  (apk install + feeds). WireGuard support on `network_interface`. One lookup
  data source per type.

## [1.0.0]

### Added
- Initial release covering the uapi 1.0 curated surface: firewall
  rules/zones/redirects, network interfaces/devices, wireless devices/
  interfaces, dhcp hosts, the system singleton, and the dhcp leases data
  source. Bearer auth, 423-retry, error-envelope decoding, import-adopts.
