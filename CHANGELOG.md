# Changelog

All notable changes are documented here. The provider mirrors uapi: provider
`x.y.*` covers the curated surface of uapi `x.y.*` (patch is the provider's own
line). Format follows Keep a Changelog.

## [Unreleased]

## [3.2.0] - TBD

Tracks uapi 3.2.0. **Upgrade the provider and uapi together.** Provider 3.1 fails
against uapi 3.2 when a `uapi_network_interface` changes `proto` or has an
`ip6prefix`. The new attributes and the list form of `ip6prefix` require uapi >=
3.2.0. Everything else works against uapi 3.0 and 3.1.

### Changed
- **`uapi_network_interface.ip6prefix` is a list of strings** on the resource and the
  data source, because netifd and LuCI read it as a list (#34). Change
  `ip6prefix = "2001:db8::/48"` to `ip6prefix = ["2001:db8::/48"]`, or the plan fails
  with a type error. The state migrates on the first plan: a string becomes the list of
  its space-separated words. This includes a state that provider 3.1 refreshed against
  uapi 3.2, which holds the array as text, such as `"[2001:db8::/48]"`. Removing the attribute from the configuration still
  clears it. uapi stores one prefix as a uci option and several as a uci list, and it
  reads both as a list.

### Added
- `uapi_network_interface.broadcast_flag`, for proto `dhcp`: uci `broadcast` on a dhcp
  interface, which makes udhcpc ask the server for broadcast replies (`-B`). `broadcast`
  stays the static broadcast address. uapi read LuCI's `broadcast '1'` on a dhcp
  interface as an address, so a PUT of such an interface answered 422.
- `uapi_network_interface.sourcefilter`, for protos `dhcpv6`, `pppoe` and `ppp`. Set it
  to `false` for a routed `ip6prefix`: the default route from upstream then has no
  source, and the downstream subnets of the prefix get a default route (#31).
- `uapi_network_interface.ip6class` and `ip6weight`, for every proto. They complete the
  prefix assignment options next to `ip6assign` and `ip6hint`.
- The new attributes are also on the `uapi_network_interface` data source.
- `uapi_wireless_device.path`, `macaddr`, `phy` and `radio`, which bind a radio to its
  hardware, and `cell_density` and `legacy_rates`. `uapi_wireless_interface.ieee80211r`,
  `ft_over_ds`, `ft_psk_generate_local`, `mobility_domain`, `nasid`,
  `reassociation_deadline`, `r0_key_lifetime`, `r1_key_holder`, `pmk_r1_push`, `r0kh` and
  `r1kh`, the 802.11r settings of LuCI's WLAN roaming tab. They are also on the data
  sources.
- `r0kh` and `r1kh` hold shared keys, so they are write-only and sensitive, like `key`:
  uapi never returns them, `has_r0kh` and `has_r1kh` say whether they are set, and the
  state keeps the configured value. Removing them from the configuration keeps the entries
  on the router, since uapi keeps them when a PUT leaves them out. Set them to `[]` to
  remove the entries. The generator now emits a
  write-only list attribute for them.

### Fixed
- **The first update of an imported `uapi_wireless_device` removed its `path`, and the
  radio stayed down** (#37). An update is a PUT, and uapi removes the options that the
  resource does not model. Without `path` (or `macaddr`, on a radio that an older
  `wifi detect` bound by MAC address), OpenWrt finds no phy for the radio. The update
  of an imported `uapi_wireless_interface` also removed its 802.11r options, and it failed
  with 422 when `encryption` carried a cipher, such as LuCI's `psk2+ccmp`. uapi 3.2.0
  models these options (openwrt-iac/uapi#238), and the new attributes are Optional+Computed,
  so a configuration that omits them keeps the imported values. Against uapi 3.1, the
  update still removes them. uapi 3.2.0 still does not model `num_global_macaddr` on a
  radio, or the 802.11k and 802.11v options of an SSID, so an update still removes those.
- **A `proto` change on `uapi_network_interface` failed against uapi 3.2 with 422.**
  The attributes of the old proto are Optional+Computed, so the plan kept their values
  from the state and the PUT sent them. Static to dhcp sent `ipaddrs` and `ip6addrs`.
  Dhcp to static sent `hostname`, `clientid` and `peerdns`. Wireguard to another proto
  sent `addresses`, `listen_port` and `nohostroute`. The plan now sets to null each
  attribute that the planned `proto` does not take, unless the configuration sets it.
  The table comes from the uapi spec. An attribute that the old proto did not take,
  and the new proto takes, plans as unknown: it read null, and the new proto can read
  a default there, such as `nohostroute = false` on wireguard. When `proto` is unknown
  at plan time, all of these attributes plan as unknown (#32).
- **An update of a `uapi_network_interface` on a proto that uapi does not model failed
  with 422 `not_in_enum`.** Such an interface, for example a `6in4` tunnel that LuCI
  created, imports and reads, but uapi 3.2 refuses a PUT of it. A PUT would also drop
  the options of the proto's handler that uapi does not model, such as `peeraddr`. An
  update that keeps such a proto is now a merge PATCH of the changed attributes, and
  null removes an attribute. A switch away from such a proto is still a PUT, which
  replaces those options. uapi refuses a switch to such a proto (#35).
- A static interface with `ip6hint` failed with "Provider produced inconsistent result
  after apply", because uapi stored `ip6hint` only on proto `dhcpv6`. uapi 3.2.0 stores
  the prefix assignment options on every proto (#31).
- Descriptions: each `uapi_network_interface` attribute that some protos do not take
  now names the protos that take it. `metric`, `defaultroute`, `delegate`, `ip4table`,
  `ip6table`, `ip6assign` and `ip6hint` say that they apply to every proto. `clientid`,
  `reqprefix`, `listen_port`, `broadcast` and `proto` describe what they hold.
- Descriptions: `family` on `uapi_mwan3_interface` and `uapi_mwan3_rule` no longer lists
  `any`, which uapi refuses. `srv_line` no longer says that `unbound-checkconf` checks
  the lines, because nothing runs it, and `srv_line` and `ext_line` say what happens when
  unbound cannot parse a line. `uapi_dhcp_host.macs` no longer names `mac` and
  `mac_aliases`, which 3.0.0 removed. `endpoint_port`, `persistent_keepalive`,
  `dhcp_option` and `uapi_lldpd_config.interface` describe what they hold (#41).

### Notes
- uapi 3.2.0 refuses, with 422 `conflict`, a value for a `uapi_network_interface`
  attribute that the proto does not take. The provider now reports it at plan time.
  Before, uapi answered 200: it kept a static address field such as `ipaddrs` on any
  proto, where netifd ignores it, and dropped every other such field. If your
  configuration sets such an attribute, remove it or set it to null. An empty list
  passes, because uapi stores nothing for it. An empty string does not pass: it reads
  back null, and the apply would fail.
- `metric`, `defaultroute`, `delegate`, `ip4table` and `ip6table` now apply to every
  proto. Before, uapi dropped them on some protos and answered 200. `peerdns` and
  `reqprefix` now also apply to `pppoe` and `ppp`, and `clientid` to `dhcpv6`.
- uapi 3.2.0 changes the ETag of every network interface, wireless device and wireless
  interface once at the upgrade, and of some routes, rules, devices, bridge VLANs and
  wireguard peers. A normal plan reads the new ETag first, so nothing fails. An apply
  with `-refresh=false`, or of a plan saved before the upgrade, answers 412 for such a
  resource. Run a plan with refresh after the upgrade.
- A PUT of a wireguard interface writes `nohostroute '0'` to uci when the option was
  unset, because uapi reads an unset `nohostroute` as false and the provider sends the
  value from its state. The value has no effect. uapi 3.2.0 stopped writing it on a
  PATCH that does not name the field, which covers the update of an interface on a
  proto that uapi does not model.
- uapi 3.2.0 refuses more writes that it accepted before. The provider adds no
  client-side check, so each one is an apply-time `422`:
  - `mtu` below 68 on `uapi_network_interface` and `uapi_network_device`. netifd
    ignores such a value. On proto `ppp` and `pppoe`, `mtu` must also be from 128 to
    16384, because pppd refuses other values.
  - `listen_port = -1` on `uapi_network_interface`. uapi reads -1 when uci holds a
    service name that does not resolve, and wireguard then cannot bring the interface
    up. Set a port.
  - `table = 0` on `uapi_network_route` and `lookup = 0` on `uapi_network_rule`,
    because netifd cannot resolve table 0.
  - A `uapi_network_rule` `goto` that the kernel refuses: with an action other than
    goto, or one that does not jump past `priority`.
- A section that already stores such a value reads it as it is, and uapi refuses every
  write to that section until the value is fixed. For `mtu` and `table`,
  `uapi_diagnostics` with `validate = true` lists these sections.
- uapi 3.2.0 also refuses, with 422, wireless, WireGuard and DHCP values that the daemon
  refuses (#41):
  - On `uapi_wireless_interface`: `mode = "wds"`, for which OpenWrt 25.12 sets up no
    interface, an `ssid` over 32 bytes, and a `key` that hostapd or wpa_supplicant
    refuses, such as one under 8 bytes with `psk2`. With 802.11r, also a `nasid` over
    48 bytes, and `r0kh` or `r1kh` entries that hostapd cannot parse.
  - On `uapi_network_wireguard_peer`: an `endpoint_host` with a `#` or a backslash, and
    a `public_key` or `preshared_key` that is not in the form that `wg pubkey` or
    `wg genpsk` prints.
  - On `uapi_dhcp_server`: a `dhcp_option` entry in which an address is followed by
    whitespace and another option, such as `3,10.0.0.1 6,10.0.0.1`. dnsmasq reads each
    entry as one option, so it refused such a line and did not start, while the apply
    passed. A space in a text option, such as `15,my domain`, still passes.

  A value that uci already holds blocks a write only when the write makes the daemon
  read it, for example when it sets the SSID up, enables the peer, or changes the
  `dhcp_option` list.
- **A write that turns on a radio or a WireGuard interface checks what it turns on**
  (#41). uapi 3.2.0 answers 422 `conflict` to:
  - a `uapi_wireless_device` write that turns the radio on, or gives it a `type`, while
    an enabled SSID of the radio holds a value that the daemon refuses.
  - a `uapi_network_interface` write that switches to proto `wireguard`, or enables a
    wireguard interface, while a peer of the interface holds a value that a peer write
    refuses.

  The message names each SSID or peer and its fields. Such a value can come from LuCI,
  from a hand edit, or from uapi 3.1, which accepted it. An SSID references its radio,
  and a peer its interface, so Terraform updates the radio or the interface first, and
  it skips the SSID or the peer when that update fails. Each new apply then fails the
  same way. Fix the SSID or the peer in an apply that keeps the radio or the interface
  off, or apply it first with `-target`.
- uapi 3.2.0 stores `mode 'ap'` when a create or a PUT of a `uapi_wireless_interface`
  sends no `mode`. With uapi 3.1, an SSID that the configuration created without `mode`
  had no mode in uci, so OpenWrt set up nothing for it until a later update sent the
  mode from the state.
- An apply of `uapi_unbound_server`, `uapi_unbound_srv` or `uapi_unbound_ext` that
  stops unbound now fails with `500 reload_failed_restored`, and uapi restores the
  previous configuration. Before, the apply passed and DNS stayed down. The next plan
  shows the same change again (#41).
- An apply of `uapi_dhcp_server`, `uapi_dhcp_host` or `uapi_dhcp_dnsmasq`, and of
  `uapi_dhcp_odhcpd` while dnsmasq runs, now fails with `500 reload_failed_restored`
  when dnsmasq refuses to parse the configuration that it gets. uapi then restores the
  previous configuration, and the next plan shows the same change again. A
  configuration that dnsmasq parses but cannot start with, such as a port that another
  daemon holds, still passes (#41).
- uapi 3.2.0 also reads some network values as netifd reads them:
  - An integer in hex or octal, such as `metric '0x10'`, reads as its value.
  - A scalar `dns`, `ipaddr`, `ip6addr`, wireguard `addresses`, device or bridge VLAN
    `ports`, or peer `allowed_ips` with several words reads as one entry per word.
  - A table name in a route or a rule reads as its id.

  The first refresh after the router upgrade can change these values in state. A plan
  shows a change only where the configuration differs from the value that netifd uses.
- uapi 3.2.0 reads more values as the daemons read them (#41):
  - `uapi_dhcp_host.macs`, `uapi_dhcp_server.dhcp_option` and
    `uapi_lldpd_config.interface`: an option of several words reads as one entry per
    word. For `macs` and `interface`, a list entry of several words also reads as its
    words. uapi refuses an entry with a space in `macs` and `interface`, so list each
    value as its own entry. Before, every update of a host with several MACs in one
    option failed with 422.
  - `uapi_network_wireguard_peer.endpoint_port` reads 0 for a service name that does
    not resolve, and `persistent_keepalive` reads -1 for a value that wg cannot read,
    such as `25x`. Before, both read null, and an update removed the option. uapi
    refuses an update that sends the 0 back, so set `endpoint_port` in the configuration
    of such a peer. A keepalive of -1 that an update sends back is stored as `-1`.

  As for the network values, the first refresh can change these values in state, and
  the ETag of the section changes.
- uapi 3.2.0 answers `503 config_missing` to a write into a package whose file under
  `/etc/config/` does not exist, and the message names the file. The common case is
  `uapi_wireless_device` and `uapi_wireless_interface` on a router without a radio.
  Before, uapi answered 200 and stored nothing, so the next plan showed the resource to
  create again.
- A live acceptance run (`UAPI_ACC_ENDPOINT` set) skips a `TestAccAllResources` case
  whose create answers `503 config_missing` or `503 init_script_missing`, with the uapi
  message as the reason. Before, such a case failed, and with uapi 3.1 a wireless case
  failed on a missing resource with no hint of the cause. (#36)

## [3.1.0] - 2026-10-11

Tracks uapi 3.1.0. The new resource and field require uapi >= 3.1.0. Everything
else works against uapi 3.0.

### Added
- `uapi_cron_job` resource and data source, wrapping `system/cron_jobs`: one line of
  the root crontab (`/etc/crontabs/root`). uapi keeps every other line byte for byte,
  so the lines that packages such as `acme` add survive. There is no update endpoint,
  so a change to `schedule` or `command` replaces the job. The id is a hash of the
  line, so `terraform import` recovers both fields. The data source also exposes
  `runtime.crond_running` and `runtime.runs_as_written`.
- The resource warns when procd reports no running crond after a write or a refresh.
  The cron init script exits 0 even when crond does not start, so a write that
  answered 200 can still leave every job idle. It also warns when an imported line
  does not run as written, such as `*/2` in the day field.
- `uapi_dhcp_host.match_tag`: dnsmasq tags that a request must carry for the
  reservation to apply (LuCI's Match Tag). Prefix a tag with `!` to negate it.

### Fixed
- **A configured empty list failed every apply with "Provider produced inconsistent
  result after apply".** uci cannot store an empty list, so uapi drops `[]` on write
  and reads the absent option back as null. Terraform requires the applied value to
  equal the configured `[]`. Because Optional+Computed lists keep their prior value
  when you remove them from config, `[]` was the only way to clear one, and it always
  failed. This affected every list attribute since 3.0.0, flat and inside `match`. A
  read now keeps a configured empty list when uapi answers null.
- `uapi_dhcp_host.tag` described the wrong direction. It sets tags on a request that
  the reservation matches (LuCI's Set Tag). It does not restrict the reservation. Use
  the new `match_tag` for that.
- `uapi_package.version` documents the full apk version. uapi 3.1.0 fixed a read that
  cut most versions to the release suffix (`3.0.0-r1` read as `r1`). The value changes
  on the first refresh after the router upgrade, with no plan diff.

### Notes
- uapi 3.1.0 refuses three `uapi_dhcp_host` writes that it accepted before. The
  provider adds no client-side check, so each one is an apply-time `422`:
  - An `ip` that another reservation holds. dnsmasq stopped on such a pair, so the
    router lost DNS and DHCP after a write that answered 200.
  - A comma, a double quote, a `#` or a control character in `name`, `tag` or
    `match_tag`. These went into dnsmasq's config line, and a newline in `name` added a
    dnsmasq directive of its own.
  - A blank value in `match_tag`.
- If your configuration sets `tag` to limit a reservation to some clients, it did not
  do that. Move those tags to `match_tag`. Only a tag that dnsmasq sets from the
  request before it looks up hosts can match, for example a tag from a `config
  vendorclass` rule. `known` never matches.

## [3.0.1] - 2026-08-13

Fixes an upgrade blocker in 3.0.0. Upgrading from 3.0.0 or from 2.x needs no
configuration change: the state migration runs automatically on the first plan.

### Fixed
- **3.0.0 could not read state written by 2.x, and `plan` stopped before producing
  anything** (#28). `uapi_firewall_redirect.match`'s address and port selectors
  changed from lists to strings in 3.0.0, correctly, since firewall4 discards a
  section that writes a uci list on those options. But the resource still declared
  schema version 0 and shipped no upgrader, so Terraform decoded 2.x state against
  the new schema and failed with a schema mismatch that named neither the field nor
  the remedy.

  Two resources now declare schema version 1 and migrate prior state:

  - `uapi_firewall_redirect` (3.0.0): each of the six `match` selectors collapses to
    its first element. A longer list never reached the router, since firewall4
    discarded any redirect writing a uci list, and the upgrade warns if it drops one.
  - `uapi_dhcp_host` (2.5.0): `tag` becomes a list if it was stored as a string,
    splitting on whitespace the way uci stores and uapi returns it.

  Only the list/scalar boundary needs an upgrader. Terraform's passthrough decoder
  coerces between JSON scalars, so a stored `true` reads back into a string
  attribute as `"true"` and `"64"` into a number: the two bool-to-string
  corrections in 2.5.0 (`system.urandom_seed`, `lldpd_config.lldp_description`) and
  the 48 string-to-number changes in 2.0.0 were never undecodable, and are left
  alone rather than routed through a hand-written upgrade path. Measured against
  `tfprotov6.RawState.Unmarshal`, not assumed.

  Version-0 state is ambiguous, since the provider never stamped a version before
  this release: it may hold the old shape or the new one. Both upgraders normalize
  rather than convert, so state written by 3.0.0 passes through untouched.

### Notes
- If you already hit this on 3.0.0, the recovery is `terraform state rm` for **every**
  affected resource first, then `import` them back. Removing and importing one at a
  time fails: each import re-reads the whole state file, so any remaining undecodable
  entry fails the import too, and a loop in the obvious order reports failure for all
  but the last. Upgrading to 3.0.1 avoids the recovery entirely.
- A type change that crosses the list/scalar boundary now requires a `SchemaVersion`
  bump in `descriptors.go` plus a `StateUpgrader`; the generator emits the version
  and `state_upgrades.go` holds the migrations.

## [3.0.0] - 2026-08-13

Tracks uapi 3.0.0. **Requires uapi >= 3.0.0**, and provider 2.x cannot talk to a
3.0 router: a uapi installation serves exactly one API major. Read the
"Migrating from provider 2.x to 3.0" guide before upgrading; it gives the order,
and most of the work is edits you can make and apply while still on 2.5.x.

### Upgrade note

- **The endpoint moves to `/api/v3`.** Point `endpoint` at the new prefix.
- **`uapi_vnstat_interface` is removed.** The `vnstat/interfaces` endpoint is gone:
  it wrote a uci section vnstat never reads. Use `uapi_vnstat_config.interfaces`,
  which is the list the daemon actually uses, and `terraform state rm` the old
  resources rather than destroying them.
- **`uapi_network_interface.ipaddr` is read-only**, a view of the first `ipaddrs`
  entry. Set `ipaddrs` instead, even for a single address. This also retires the
  provider-side machinery that kept the pair consistent through 2.5.x.
- **`uapi_network_interface.name` is removed.** The deprecation window opened in
  2.2.0 and closes here; use `id`.
- **`uapi_dhcp_host.mac` and `mac_aliases` are removed.** Use `macs`.
- **`uapi_firewall_redirect`'s match selectors are scalars**, not one-element lists:
  `src_ip`, `src_port`, `src_dport`, `src_dip`, `dest_ip` and `dest_port`. Change
  `src_dport = ["8443"]` to `src_dport = "8443"`, and so on. `proto` stays a list.
  firewall4 parses a `config redirect` option as a scalar, and through 2.x these
  were arrays capped at one entry, so a second value was an apply-time `422`; as
  scalars the same mistake is a plan-time type error (openwrt-iac/uapi#148).
- **28 attributes that wrote a uci option nothing reads are removed**: all 18
  collector toggles on `uapi_prometheus_node_exporter_lua_config`,
  `uapi_vnstat_config.database_dir` / `interface_5min_hours` / `month_rotate`,
  `uapi_mwan3_globals.local_source` / `rtmon_interval`,
  `uapi_unbound_server.enabled` / `prefetch`, `uapi_usteer_config.max_assoc_sta`,
  and `uapi_lldpd_config.enable_lldpmed`. Provider 2.5.0 warned about each of them
  at plan time. Removing them from configuration changes nothing on the router.

### Changed
- **An absent uci list now reads as null rather than an empty list.** uapi 3.0 makes
  that distinction on the wire and the provider follows it, because unset is not the
  same as set-to-empty. On the first plan after upgrading, a list attribute you have
  never configured may show as moving from `[]` to null; it converges in one apply
  and no resource is recreated.
- `uapi_network_interface` gains `broadcast`, `ip6addrs`, `ip6gw` and `ip6prefix`,
  so an IPv6-addressed static interface is expressible.

### Added
- uapi's `X-Mgmt-Path-Warning` is now surfaced as a Terraform warning. uapi sets it
  when a write touches the interface the request arrived through, and the provider
  was discarding it, so the first sign was the router no longer answering. It now
  appears against the resource on the apply that causes it, carrying uapi's own
  explanation. It is advisory: the write proceeds.

  uapi 3.0 widened the header past `network/interfaces` (openwrt-iac/uapi#132, filed
  after a `network/bridge_vlans` write took a test router off the network with no
  warning available). The provider reads it from every write response, so it covers
  whatever uapi emits it on.

### Fixed
- A create refused for any `conflict` field error reported it as an id collision.
  The hint told the reader to `terraform import` the section or pick a different
  `id`, but uapi spends that code on more than identity: a bridge VLAN naming a
  device that does not exist answers `{field: "device", code: "conflict"}`, and the
  advice sent the reader after a section that never existed. The hint is now
  limited to the `id` and `name` fields; every other conflict shows uapi's own
  message, which says what is actually wrong.
- The idempotency key on a create never reached uapi, so a retried `POST` could
  double-create. It was sent as the `Idempotency-Key` header only, and uhttpd's CGI
  layer forwards a fixed header allowlist that the header is not on, exactly the
  reason `If-Match` has been sent as `?if_match=` since 1.0. Both now go on the
  query string as well. uapi 3.0.0-rc3 declaring these fallbacks as spec
  parameters is what surfaced it; the unit test asserted the header, which passed
  while the guarantee was inert against a real router.

### Notes
- The generated resources now come from uapi's split request and response schemas.
  The response supplies the attribute set, the request supplies required-ness and,
  by what it omits, which fields are writable at all. That is how `ipaddr` became
  computed without a hand-written exception.
- The acceptance fixture for `uapi_network_bridge_vlan` no longer targets `br-lan`.
  Creating a bridge VLAN on the management bridge enables VLAN filtering on it and
  takes a live target off the network mid-suite, which is how the warning gap above
  was found. It now creates its own portless bridge instead of naming one it hopes
  exists, and the `uapi_network_wireguard_peer` fixture carries a real 44-char key
  and the `addresses` a wireguard interface requires. Both were accepted by the fake
  and refused by a real router, which is the third instance of that class in this
  suite.

## [2.5.1] - 2026-08-10

Documentation only. No schema, behaviour or dependency change, so upgrading from
2.5.0 needs nothing.

### Fixed
- Eleven attribute descriptions described the wrong thing. Descriptions are keyed
  by field name, so a name meaning different things on different resources got one
  generic sentence, and for these it was misleading rather than merely vague. Found
  by running the acceptance suite against a real router, where
  `uapi_openvpn_instance.key` was rejected with "must be an absolute filesystem
  path" while the docs called it a passphrase.
  - `uapi_openvpn_instance.key`, `tls_auth` and `pkcs12`, and
    `uapi_uhttpd_instance.key`, are **paths to files on the router**, not the key
    material. They were documented as passphrases and bundles.
  - `uapi_network_route.target` is the **destination address or CIDR**. It was
    documented as "Target / action.", which is the firewall meaning of the word.
  - `uapi_dropbear_instance.interface` is the interface or IP dropbear **listens
    on**, and `uapi_mwan3_member.interface` names an `uapi_mwan3_interface` section
    rather than a network interface.
  - `name` on `uapi_dhcp_host`, `uapi_firewall_zone` and `uapi_network_device` is
    the hostname, the zone name and the kernel device name respectively. All three
    were documented as "Optional section name.", which understates what they do.

  `uapi_network_interface.private_key` and `uapi_network_wireguard_peer.preshared_key`
  are unchanged: those really are key material.

### Notes
- The acceptance fixture for `uapi_openvpn_instance` passed a PEM header where a
  path belongs. The fake accepted it and a real router did not, which is what
  surfaced the documentation bug; the fixture now uses a path.
- `CONTRIBUTING.md` and the live-acceptance workflow now warn that the
  `uapi_network_*` cases reconfigure the interfaces the run arrives over, and can
  take a remote box off the network with a section that stays committed in uci.

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
  gated on openwrt-iac/uapi#16.
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
  upstream at openwrt-iac/uapi#16.)

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
