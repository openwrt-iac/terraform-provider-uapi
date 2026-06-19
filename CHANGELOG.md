# Changelog

All notable changes are documented here. The provider mirrors uapi: provider
`x.y.*` covers the curated surface of uapi `x.y.*` (patch is the provider's own
line). Format follows Keep a Changelog.

## [Unreleased]

## [2.2.3] - 2026-06-19

Tracks uapi 2.2.3. Consumes the new `x-uapi-clear-on-omit` spec annotation so
caller-owned, non-defaulted fields can be cleared by removing them from config.

### Changed
- `uapi_network_interface.netmask` and `gateway` are now plain `Optional` (were
  `Optional + Computed`). Removing one from config plans it to null (an in-place
  update, not a replacement) and clears the uci option, which is how you drop a
  leftover static field on an interface adopted into `proto=dhcp`. Other optional
  fields are unchanged, and server-defaulted fields stay sticky (no perpetual
  diffs).

### Upgrade note
- Run `terraform plan` after upgrading. A managed or adopted `uapi_network_interface`
  with `netmask` or `gateway` in state but not in config will plan those to null on
  the first plan, then converge on apply. Interfaces that set them in config see no
  change. (`ipaddr`/`ipaddrs`/`dns` are not yet clearable this way; tracked
  upstream at openwrt-iac/uapi#3.)

## [2.2.1] - 2026-06-18

Tracks uapi 2.2.1 (a validate-only patch; no schema change versus 2.2.0) and
resolves the provider-side follow-ups from a full 125-resource production apply.

### Fixed
- `uapi_network_interface` migration from the deprecated `name = "x"` to
  `id = "x"` is now a non-destructive in-place update (the `name` attribute
  clears from state), not a destroy + recreate. A real rename (changing `id`, or
  setting `name` to a different value) still replaces.
- A create whose `id`/`name` collides with an existing section (uapi returns
  `422 validation_failed` with a `conflict` field error) now surfaces a hint to
  `terraform import` the section or choose a different `id`, instead of a bare
  validation error.

### Changed
- Docs: `uapi_sqm_queue.interface` / `uapi_vnstat_interface.interface` are
  documented as a network interface name (not a kernel device); firewall `target`
  values are noted as case-sensitive upper-case (`ACCEPT`/`REJECT`/`DROP`/
  `NOTRACK`/`MARK`; `DNAT`/`SNAT`); `uapi_firewall_zone`/`uapi_dhcp_server`/
  `uapi_sqm_queue` also ship as box defaults and should be `terraform import`ed.

## [2.2.0] - 2026-06-12

Tracks uapi 2.2.0. Fixes a design dead-end where a pre-existing named section
(`lan`/`wan`/`br-lan`) had no safe management path. Requires uapi >= 2.2.0.

### Added
- Settable `id` on every collection resource: set it to choose the uci section
  name (e.g. `id = "lan"`), or omit it for a server-assigned ULID. Create-only
  (changing it forces replacement), never sent on update.
- Adopt-keep-name: `terraform import` of a named section keeps its name (no rename
  to a ULID) and does not mutate the router, so config with the same `id`
  reconciles with no replacement. Anonymous `cfgXXXX` sections still adopt by
  renaming (with a warning).
- `uapi_dhcp_host.ip` is now optional: omit it for a `mac`+`name` DNS-only
  reservation (no static lease).
- `uapi_network_device` of `type = "bridge"` no longer requires `ports`.
- Actionable hint on a create conflict, pointing at `terraform import`.

### Deprecated
- `uapi_network_interface.name` in favour of the universal `id` (both accepted
  through v2; removal targeted v3).

## [2.1.0] - 2026-06-07

Tracks uapi 2.1.0. Requires uapi >= 2.1.0.

### Added
- `uapi_unbound_srv` and `uapi_unbound_ext` singletons (`interface_bind`,
  `interface_outgoing`, `srv_line`, `ext_line`) for loopback-only recursive
  setups, multi-WAN egress, and verbatim unbound config.

### Changed
- BREAKING: the provider moved to the `openwrt-iac` namespace; its source address
  is now `openwrt-iac/uapi` (was `raspbeguy/uapi`). Schema, resource set, and
  ULID `id`s are unchanged. To upgrade: update `source`, run
  `terraform init -upgrade`, then
  `terraform state replace-provider registry.terraform.io/raspbeguy/uapi registry.terraform.io/openwrt-iac/uapi`.

## [2.0.1] - 2026-06-06

Tracks uapi 2.0.2 and follows up on field feedback from a real migration.
Requires uapi >= 2.0.2.

### Added
- Create-time `name` on `uapi_network_interface`: picks the uci section name,
  fixing WireGuard interfaces (the ULID section name exceeded `IFNAMSIZ` so
  tunnels never came up). When omitted, the server emits a short `wg_<rand>` for
  `proto=wireguard` or a ULID otherwise.
- `uapi_package.pre_existed`: `terraform destroy` no longer uninstalls a package
  that was already installed before Terraform managed it.

### Changed
- The 423/429 lock retry is time-bounded (with backoff + jitter) rather than
  attempt-bounded, so the default Terraform parallelism drains through the
  per-package lock instead of exhausting a small retry count.
- Docs: firewall-rules and referencing-resources guides, SQM units, a minimal
  SNMP v2c example, and a daemon-package-ordering note.

## [2.0.0] - 2026-06-05

Tracks the uapi 2.0 `/api/v2` surface. Requires uapi >= 2.0.1.

### Added
- The curated CRUD/singleton resources and lookup data sources are now generated
  from the vendored uapi OpenAPI spec (spec-driven codegen), with strict integer
  and snake_case field handling.
- `uapi_token` ephemeral resource (mint on open, revoke on close); `whoami` /
  `healthz` / `diagnostics` operational data sources; mwan3, usteer, and openvpn
  resources.
- Client handling for 429 rate limiting, cursor pagination, and an
  `Idempotency-Key` on every create.

### Changed
- BREAKING: a new major tracking uapi 2.0. The provider talks to `/api/v2` (the
  major version lives in the user-supplied `endpoint` path).

## [1.2.0] - 2026-06-03

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

## [1.1.0] - 2026-05-31

### Added
- Full uapi 1.1 curated surface: 16 CRUD resources (network routes/rules/
  bridge_vlans/wireguard_peers, firewall forwardings, dhcp servers, snmpd
  accesses/agents/com2secs/groups, sqm queues, system timeservers, uhttpd
  certs/instances, vnstat interfaces), 8 singletons, and `packages/*`
  (apk install + feeds). WireGuard support on `network_interface`. One lookup
  data source per type.

## [1.0.0] - 2026-05-30

### Added
- Initial release covering the uapi 1.0 curated surface: firewall
  rules/zones/redirects, network interfaces/devices, wireless devices/
  interfaces, dhcp hosts, the system singleton, and the dhcp leases data
  source. Bearer auth, 423-retry, error-envelope decoding, import-adopts.
