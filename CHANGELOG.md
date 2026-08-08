# Changelog

All notable changes are documented here. The provider mirrors uapi: provider
`x.y.*` covers the curated surface of uapi `x.y.*` (patch is the provider's own
line). Format follows Keep a Changelog.

## [Unreleased]

## [2.5.0] - 2026-08-09

Tracks uapi 2.5.0. Requires uapi >= 2.5.0.

### Upgrade note (read this first)

Three attributes change type. All three were modelling mistakes uapi corrected; the
old shapes never described what the daemon actually reads. **Two of them fail
quietly**, so grep your configuration rather than relying on `terraform plan` to
catch them.

- **`uapi_dhcp_host.tag` is now a list** (was a string). uapi always answers with an
  array, including for a section stored as a space-separated scalar, so the
  provider has to match the response. Change `tag = "red"` to `tag = ["red"]`, and
  a space-separated `tag = "red blue"` to `tag = ["red", "blue"]`. This one is
  loud: an unedited config fails at plan time with "list of string required".
- **`uapi_lldpd_config.lldp_description` is now a string** (was a boolean), the
  system description advertised in LLDP frames. Set the text you want:
  `lldp_description = "edge router"`.
- **`uapi_system.urandom_seed` is now a string** (was a boolean), the path the
  entropy seed is saved to, e.g. `urandom_seed = "/etc/urandom.seed"`.

  These two fail quietly. HCL coerces a bare `true` to the string `"true"`, so an
  unedited config plans and applies without complaint and writes `"true"` as your
  LLDP description, or as the seed path where the reader only acts on a value
  starting with `/`. Nothing errors; the setting is simply wrong. Search your
  configuration for both attributes before upgrading.

### Added
- `uapi_dhcp_host.macs`, the uci `list mac`, replacing the now-deprecated `mac` and
  `mac_aliases`. It takes precedence over both when non-empty.
- `uapi_vnstat_config.interfaces`, the devices vnstat tracks, named as the kernel
  names them (`br-lan`, `eth0`). uapi reports this is the only vnstat option any
  shipped code reads, and it was previously unreachable through the API.
- `uapi_diagnostics` gains an optional `validate` argument that runs uapi's
  validation sweep, plus `invalid_sections`, `swept_resources` and
  `skipped_for_scope` to report it. With `validate = true` the router lists every
  section a write would reject today, managed by Terraform or not, which is how you
  find configuration broken by a uapi validation change before an apply hits it.
  It is off by default because the sweep re-validates every section on each read.
  An empty `invalid_sections` beside a non-empty `skipped_for_scope` means the token
  was not allowed to look, not that nothing is wrong.
- `uapi_diagnostics.management_path` reports which interface the request arrived
  through, so a config that would move that interface can be spotted first.
- `disabled` on `uapi_network_interface`, `uapi_network_route` and
  `uapi_network_rule`. uapi did not model it, so a disabled section read back as
  active and Terraform would have re-enabled it on the next write.

### Deprecated
- 29 attributes are marked deprecated and now raise a Terraform warning at plan
  time, carrying uapi's own explanation of why each one is dead. uapi audited every
  curated field against its reader in the OpenWrt sources and found these write a
  uci option nothing reads, so setting them has never had any effect. They are
  scheduled for removal in v3: `uapi_dhcp_host.mac` and `mac_aliases` (use `macs`),
  18 collector toggles on `uapi_prometheus_node_exporter_lua_config`,
  `uapi_vnstat_config.database_dir` / `interface_5min_hours` / `month_rotate`,
  `uapi_mwan3_globals.local_source` / `rtmon_interval`,
  `uapi_unbound_server.enabled` / `prefetch`, `uapi_lldpd_config.enable_lldpmed`,
  and `uapi_usteer_config.max_assoc_sta`.

### Notes
- uapi marks `managed` read-only across every schema in 2.5.0. No provider change:
  it was already a computed attribute and was never sent on a write.

## [2.4.1] - 2026-08-03

A provider-side bugfix release. It tracks no new uapi surface and needs no
particular uapi patch: the provider now sends only one of the two names, so the fix
does not depend on which one the server prefers and applies against uapi 2.4.0 just
as well as 2.4.1.

### Fixed
- Changing `uapi_network_interface.ipaddr` no longer fails or silently applies the
  previous address. `ipaddr` and `ipaddrs` are two wire names for one uci option
  (`list ipaddr`) and uapi fills both from that single key on read, so whichever one
  a config did not set was pinned in state and sent back on the full-replace PUT.
  Because uapi prefers the list, a stale pinned `ipaddrs` overrode a changed
  `ipaddr`: the write returned 200, uci kept the old address, and Terraform failed
  the apply with "Provider produced inconsistent result after apply". Managing
  `ipaddrs` and changing it hit the mirror image of the same problem, which uapi
  briefly rejected with a `422` before fixing it upstream
  (openwrt-iac/uapi#65). Both directions now work, and an update that touches
  neither still round-trips the address rather than clearing it, which remains
  gated on openwrt-iac/uapi#3.
- `uapi_network_interface.ipaddr` and `ipaddrs` documentation now explains that they
  are one uci option and that you should set one or the other, replacing the
  placeholder `uci option ipaddr.` rows.

### Notes
- The pair is modelled with a sibling-aware plan modifier rather than
  `UseStateForUnknown`: the unset side plans as unknown when the sibling is
  configured (so the server can recompute it and the request omits the stale value),
  and falls back to prior state when neither is configured (so an unrelated update
  preserves the address). New resources do not need this; it applies only where the
  API exposes one value under two names.

## [2.4.0] - 2026-07-31

Tracks uapi 2.4.0, which closes the gap between what the firewall resources
advertise and what firewall4 actually applies. Requires uapi >= 2.4.0 for the new
resource and fields.

### Added
- `uapi_firewall_nat` resource and data source, wrapping `config nat`: the way to
  express MASQUERADE or an exemption from source NAT. `target` is `SNAT` (with
  `snat_ip` / `snat_port`), `MASQUERADE`, or `ACCEPT`. Its nested `match` is a third
  match shape: nested like a rule, but with **scalar** addresses and ports, because
  firewall4 parses a `config nat` section's options as scalars. Only `proto` is a
  list.
- `uapi_firewall_rule` gains `set_mark`, `set_xmark`, and `set_dscp`, plus the `DSCP`
  target. A `MARK` or `DSCP` target with no value to set is now a `422` instead of a
  rule the router silently discards.
- `uapi_firewall_rule` gains `match.mark` and `match.dscp`; `uapi_firewall_redirect`
  gains `match.mark`. Each accepts a leading `!` to negate.
- `uapi_firewall_redirect` gains `match.src_dip`, the address firewall4 rewrites the
  source to on an SNAT redirect and matches the external destination against on a
  DNAT one. It was the only mandatory option of an SNAT redirect that was not
  modelled, and because updates are a full-replace PUT, its absence was destructive:
  a plain read-modify-write of a working section dropped it. SNAT redirects are
  writable for the first time.
- `runtime.effective_proto` on the `uapi_network_interface` data source: the protocol
  netifd is actually running. It differs from the configured `proto` when the handler
  package is missing (`wwan` is the case that arises in practice), where the write
  succeeds, uci keeps the value, and the interface is inert. Comparing the two fields
  is the only way to detect that.

### Changed
- `uapi_firewall_rule.match.src_zone` is now `Optional` (was `Required`). firewall4
  needs a source zone only for `NOTRACK`; a rule without one is valid and lands in
  the `output` chain. `uapi_firewall_redirect.match.src_zone` stays required.

### Fixed
- The provider `endpoint` documentation example now shows `/api/v2` instead of the
  stale `/api/v1`, matching the major this provider line covers. Docs only, no
  behavior change: the version prefix comes from the user-supplied endpoint.
- `make install` writes to the `openwrt-iac` plugin namespace instead of the
  pre-rename `raspbeguy` one, so a dev override built from source resolves.

### Notes
- uapi 2.4.0 tightens firewall validation, and configurations that previously
  returned 200 while the router discarded the section are now rejected with a `422`.
  The provider adds no client-side validation for any of it (enums and grammars are
  validated server-side so uapi can widen them without a provider release), so these
  surface as apply-time errors. Run `terraform plan` and expect the firewall guide's
  new sections to explain them. The one to know about is a port matched alongside a
  protocol that cannot carry one: that was a silent **widening**, not a no-op, and
  `proto = ["all"]` with a `dest_port` rendered a rule matching everything.
- A redirect's `match` fields stay lists on the wire but now accept at most one value
  each; a second is a `422`. No provider type change.
- uapi 2.4.0 leaves the `match` block itself optional on `firewall/rules` and
  `firewall/nat` (only `firewall/redirects` still requires it), but the provider
  requires it on all three. Write `match = {}` for a section that matches
  everything. Terraform's type system is the reason: the block is modelled as a
  nested struct, and making it optional would plan it as unknown, which that
  representation cannot hold.
- The repaired `openvpn` `dev_type` / `proto` enums and the `readOnly` flag now set on
  the `runtime` blocks need no provider change: the provider does no client-side enum
  validation and the generator never modelled `runtime` as writable.

## [2.3.0] - 2026-06-24

Tracks uapi 2.3.0.

### Added
- `uapi_token` ephemeral resource gains optional `rate` and `burst`. They set the
  minted token's per-token rate limit (requests per second) and token-bucket
  capacity; omitting them inherits the server's global limits.

### Changed
- `uapi_network_device.type` is now `Optional` (was `Required`). uapi 2.3.0 no longer
  requires `type` on a `config device` section, so a stock options-override section
  (name + macaddr, no type, as `config_generate` emits on some targets) round-trips.

### Notes
- uapi 2.3.0's PATCH change (it no longer drops uci options a resource does not model)
  and its relaxed validation (empty/`00` wireless country, `owe` encryption,
  type-less network devices) need no provider change: the provider reads full
  responses back into state and does no client-side enum validation. The PATCH fix
  also stops `uapi_system` (the only PATCH resource) from clobbering unmodeled options.
- Commit-confirmed apply is not included. uapi has postponed its apply-confirm
  integration to a future release (2.4.0 at the earliest, contingent on the
  apply-confirm package maturing), so there is nothing for the provider to consume.
  When it returns, the per-write form does not fit Terraform's apply model; the
  provider would track the standalone arm endpoint instead.

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
