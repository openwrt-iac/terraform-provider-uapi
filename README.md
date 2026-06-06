# terraform-provider-uapi

A Terraform / OpenTofu provider for [uapi](https://github.com/raspbeguy/uapi), the native HTTP REST API for OpenWrt.
It manages OpenWrt configuration (firewall, network, wireless, DHCP, system) through uapi's
curated endpoints, which expose stable resource IDs and atomic, transactional writes.

## Scope

This provider targets the **curated** uapi endpoints only. It deliberately does **not** use the
`/raw/<package>/<id>` passthrough: raw payloads follow uci's field names directly and carry no
stability promise across OpenWrt releases, which is a poor fit for managed Terraform state.

## Requirements

- An OpenWrt router running **uapi >= 2.0.2** (OpenWrt 25.12+), reachable over HTTP(S).
- A bearer token created on the router: `uapi-token create --name terraform --scope '*:rw'`.
- Terraform >= 1.0 (>= 1.10 for the `uapi_token` ephemeral resource) or OpenTofu >= 1.11.

> **OpenTofu users:** the provider is published on `registry.terraform.io`. Until it is also on
> the OpenTofu registry, set a `dev_overrides` / mirror so `tofu init` resolves
> `raspbeguy/uapi` (see `examples/dev.tfrc`).

> **Daemon packages first.** Resources whose daemon ships as a separate OpenWrt package
> (`uapi_unbound_server`, `uapi_sqm_queue`, `uapi_snmpd_*`, `uapi_openvpn_instance`,
> `uapi_mwan3_*`, `uapi_vnstat_*`, `uapi_lldpd_config`) need that package installed first, or
> the write returns `503 init_script_missing`. Install it out of band, or manage it with
> `uapi_package` and `depends_on = [uapi_package.<name>]`.

## Provider configuration

```hcl
provider "uapi" {
  endpoint = "https://192.168.1.1/api/v2" # or env UAPI_ENDPOINT / UAPI_BASE
  token    = var.uapi_token               # or env UAPI_TOKEN
  insecure = true                         # or env UAPI_INSECURE=1
}
```

| Argument   | Env                          | Description                                                        |
|------------|------------------------------|--------------------------------------------------------------------|
| `endpoint` | `UAPI_ENDPOINT`, `UAPI_BASE` | API root including the `/api/v2` prefix.                            |
| `token`    | `UAPI_TOKEN`                 | Bearer token. Sensitive.                                           |
| `insecure` | `UAPI_INSECURE`              | Skip TLS verification. Needed for uapi's default self-signed cert. |

> **TLS:** uapi ships a self-signed certificate. `insecure = true` (or the marker file
> `/etc/uapi.insecure` on the router) gets you going quickly; for production, install a real
> certificate via `acme.sh` / `luci-app-acme` and leave `insecure` off.

## Resources

The provider covers the full curated uapi surface (no `/raw`). The per-resource pages under `docs/`
(and on the Terraform Registry) document every attribute; the groups below are a map.

- **Firewall:** `uapi_firewall_rule`, `uapi_firewall_zone`, `uapi_firewall_redirect`,
  `uapi_firewall_forwarding`, `uapi_firewall_defaults` (singleton).
- **Network:** `uapi_network_interface` (incl. WireGuard: write-only `private_key`),
  `uapi_network_device`, `uapi_network_route`, `uapi_network_rule`, `uapi_network_bridge_vlan`,
  `uapi_network_wireguard_peer` (write-only `preshared_key`).
- **Wireless:** `uapi_wireless_device`, `uapi_wireless_interface` (write-only `key`).
- **DHCP/DNS:** `uapi_dhcp_host`, `uapi_dhcp_server`, `uapi_dhcp_dnsmasq` (singleton),
  `uapi_dhcp_odhcpd` (singleton), `uapi_unbound_server` (singleton).
- **System:** `uapi_system` (singleton), `uapi_system_timeserver`, `uapi_dropbear_instance`,
  `uapi_uhttpd_instance`, `uapi_uhttpd_cert`, `uapi_lldpd_config` (singleton),
  `uapi_authorized_key` (root SSH keys), `uapi_system_password` (write-only password set).
- **SNMP:** `uapi_snmpd_system` (singleton), `uapi_snmpd_com2sec`, `uapi_snmpd_group`,
  `uapi_snmpd_access`, `uapi_snmpd_agent`.
- **Traffic/metrics:** `uapi_sqm_queue`, `uapi_vnstat_interface`, `uapi_vnstat_config` (singleton),
  `uapi_prometheus_node_exporter_lua_config` (singleton).
- **mwan3:** `uapi_mwan3_interface`, `uapi_mwan3_member`, `uapi_mwan3_policy`, `uapi_mwan3_rule`,
  `uapi_mwan3_globals` (singleton). Requires the `mwan3` package on the router.
- **usteer / OpenVPN:** `uapi_usteer_config` (singleton), `uapi_openvpn_instance` (write-only
  `key`/`tls_auth`/`pkcs12`). Require the `usteer` / `openvpn` packages respectively.
- **Packages:** `uapi_package` (apk install/remove), `uapi_package_feed`. These have no update
  endpoint, so changing an input forces replacement.

### Ephemeral resources

- `uapi_token`: mints a scoped bearer token for the duration of a run and revokes it on close. The
  cleartext token is exposed only as an ephemeral value (never written to state), so it can feed a
  second, scoped provider instance or a downstream ephemeral consumer. Requires Terraform 1.10+ or
  OpenTofu 1.11+.

## Data sources

Every resource type has a matching lookup data source (by `id`, or no argument for singletons), plus:

- `uapi_dhcp_leases`: active IPv4 DHCP leases reported at runtime (read-only, list).
- `uapi_dhcp_leases6`: active IPv6 (odhcpd) leases (read-only, list).
- `uapi_whoami`: identity and scopes of the configured token.
- `uapi_healthz`: liveness/readiness of the service and its dependencies.
- `uapi_diagnostics`: loaded resources, uptime, and lock state.

The `uapi_network_interface` and `uapi_wireless_interface` data sources also expose a computed
`runtime` block (live ubus state: interface up/addresses/routes, wireless signal/assoclist). It is
read-only observed state, surfaced only on the data sources, never on the resources.

## Worked examples

The registry "Guides" section has worked examples for the patterns that are easy to get wrong:
the **firewall rules / redirects / forwardings** guide (the nested `match` vs flat distinction)
and the **referencing other resources** guide (`.id` vs kernel-name references, and destroy
semantics).

A minimal SNMP v2c read-only setup wires com2sec -> group -> access (each references the
previous), and needs the `snmpd` package installed:

```hcl
resource "uapi_snmpd_com2sec" "ro" {
  secname   = "ro"
  source    = "192.168.1.0/24"
  community = "public"
}

resource "uapi_snmpd_group" "ro" {
  group   = "ro_group"
  version = "v2c"
  secname = uapi_snmpd_com2sec.ro.secname
}

resource "uapi_snmpd_access" "ro" {
  group   = uapi_snmpd_group.ro.group
  version = "v2c"
  level   = "noauth"
  read    = "all"
}
```

## Behaviour notes

- **Stable IDs.** uapi assigns every managed section a prefixed ULID (e.g. `r_01HX...`) that
  survives reorders and rewrites; that ID is the Terraform resource `id`. A managed section's
  *name* is that ULID, so when one resource references another by name, point it at the
  referenced resource's `id`, not a hand-picked label:

  ```hcl
  resource "uapi_network_interface" "lab" {
    proto  = "static"
    ipaddr = "192.168.250.1"
  }
  resource "uapi_dhcp_server" "lab" {
    interface = uapi_network_interface.lab.id # the interface's name IS its ULID
  }
  ```

  Pre-existing *unmanaged* sections created out of band (LuCI/SSH/`wifi config`, e.g.
  `lan`, `wan`, `radio0`) keep their human name and are referenced by it
  (`match = { src_zone = "lan" }`).
- **Server-defaulted fields** (booleans, enum fallbacks, etc.) are modeled as
  `Optional + Computed`, so omitting them in config does not produce perpetual diffs.
- **423 / 429 retries and parallelism.** uapi serializes writes per uci package (`423` locked)
  and rate-limits clients (`429`), both with `Retry-After`. The provider retries automatically
  (time-bounded with exponential backoff + jitter) and sends an `Idempotency-Key` on every create
  so a retried `POST` cannot duplicate a resource. Because writes to the same package serialize,
  high Terraform parallelism is counterproductive there; for a large fleet of same-package
  resources (e.g. many `uapi_dhcp_host`), run with **`-parallelism=1`** to avoid lock churn.
- **Cursor pagination.** Collection reads follow the server's cursor automatically, so large
  lists are returned in full.
- **Write-only secrets.** `uapi_wireless_interface.key`, `uapi_network_interface.private_key`, and
  `uapi_network_wireguard_peer.preshared_key` are never returned by the API; the provider keeps the
  configured value and exposes a `has_*` computed flag. `uapi_system_password.password_wo` is a
  true write-only attribute (never stored in state; bump `password_wo_version` to re-apply).
- **Optimistic concurrency (ETag / If-Match).** Each resource tracks an `etag`; updates and deletes
  send it as `If-Match`, so a change made out of band (e.g. via LuCI) since the last refresh fails
  with a clear "changed outside Terraform" error (HTTP 412) instead of silently clobbering.

### `uapi_system` is a singleton

It cannot be created or destroyed. `terraform apply` writes the settings via `PATCH`; `terraform
destroy` only drops it from state and leaves the router's settings untouched.

### Importing adopts unmanaged sections

Pre-existing anonymous uci sections (created by LuCI, SSH, etc.) surface as `managed = false`.
Running `terraform import` on such a section **adopts** it: uapi renames it to a stable ULID and
flips it to managed. This means import is a *mutating* operation for unmanaged sections. When this
happens the provider emits a warning naming the old and new ids, so the rename is not silent.

```sh
terraform import uapi_firewall_rule.example r_01HX...   # already-managed: read-only import
terraform import uapi_firewall_rule.example cfg0a1b2c   # anonymous: adopted, id becomes a ULID
```

> ⚠️ Be careful importing/editing `uapi_network_interface` for the interface that backs your
> management connection. uapi only observes the init script's exit code, not runtime convergence,
> so a bad change can lock you out.

## Versioning and compatibility

**The provider version tracks the uapi version.** Provider `x.y.*` covers exactly the curated
surface of uapi `x.y.*`: the major and minor components mirror uapi, and the patch component is the
provider's own (bugfixes and provider-internal changes that do not change the covered surface). So
provider `1.2.*` targets the resources, fields, and endpoints of uapi `1.2.*`.

This works because uapi keeps a version additive within its major: uapi `1.y` is a superset of
`1.(y-1)` (new endpoints, optional fields, response fields, error codes, scope names, enum values),
and only breaking changes bump the major (`/api/v2/`).

What that means in practice:

- **Match the provider's `x.y` to your router's uapi `x.y`.** A provider built for uapi `1.y` also
  works against any newer uapi `1.z` (z >= y), because added response fields are ignored. Pointing
  a newer provider (`1.y`) at an older router (`1.z`, z < y) is the unsupported direction: resources
  or fields the provider expects may not exist there.
- **Forward compatible within a major by construction.** Responses are decoded into a map and only
  known fields are read into state, so response fields a newer uapi adds are ignored, not errors.
- **Enum values are documented, not enforced client side.** Fields like `target`, `proto`, and
  `encryption` are plain strings validated by uapi, so values uapi adds within a major work without
  a provider release (a provider release just documents them).
- **Errors are handled by HTTP status, not by the `code` string.** New error codes are surfaced
  verbatim in diagnostics but never change behaviour.
- **A breaking uapi major (`v2`, served at `/api/v2/`) maps to a provider `2.*`.** Point `endpoint`
  at the matching `/api/vN` segment for the provider major you run.

## Building and local development

```sh
make build            # build ./terraform-provider-uapi
make install          # install into ~/.terraform.d/plugins for dev_overrides
make test             # unit tests
make fmt vet          # format and vet
make gen              # regenerate the curated resources/data sources from the OpenAPI spec
make docs             # regenerate code (gen) then docs/ from schemas + examples
```

The curated resources and data sources are **generated** from the vendored uapi
OpenAPI spec (`internal/gen/openapi.json`) plus a hand-maintained descriptor
overlay (`internal/gen/descriptors.go`), which supplies the semantics the spec
cannot express (required-ness, nested `match` shapes, the runtime sub-blocks,
Optional-vs-Optional+Computed). Field names, types, `readOnly`, and `writeOnly`
come straight from the spec, so a uapi minor that adds fields or flips a string
to an integer is picked up by re-running `make gen`. The generator is
idempotent; CI fails if the committed output drifts from a fresh run. The
hand-written specials (`package`, `package_feed`, `system_password`,
`authorized_key`, the leases lists, and the `token`/`whoami`/`healthz`/
`diagnostics` surface) are not generated.

The `docs/` tree is generated by `tfplugindocs` from the schema attribute
descriptions and the snippets under `examples/`. `make docs` runs the code
generator first, then tfplugindocs; commit the result, since the Terraform
Registry renders these pages.

To try it against a router, use a CLI config with `dev_overrides` (see `examples/dev.tfrc`).
`examples/` is laid out one snippet per resource/data source for doc generation,
so combine the provider block with a resource snippet to get a runnable config:

```sh
make install
export TF_CLI_CONFIG_FILE=$PWD/examples/dev.tfrc   # edit the path inside first
export UAPI_ENDPOINT=https://192.168.1.1/api/v2 UAPI_TOKEN=... UAPI_INSECURE=1

mkdir -p /tmp/uapi-dev
cat examples/provider/provider.tf \
    examples/resources/uapi_firewall_rule/resource.tf > /tmp/uapi-dev/main.tf
cd /tmp/uapi-dev && terraform plan -var uapi_token="$UAPI_TOKEN"
```

Acceptance tests (`make testacc`) require a live uapi instance and `TF_ACC=1`.

## Releasing (Terraform Registry)

Releases are built by GoReleaser and signed for the Terraform Registry. The
`.github/workflows/release.yml` workflow triggers on a `v*` tag and produces the
artifacts the registry expects: per-platform zips, a `_SHA256SUMS` file, a GPG
detached signature of it, and `terraform-registry-manifest.json`.

One-time setup before the first release:

1. Generate a GPG key (`gpg --full-generate-key`) and add it to your
   [Terraform Registry account](https://registry.terraform.io) under the
   publisher's GPG keys.
2. Add two repository secrets:
   - `GPG_PRIVATE_KEY`: the ASCII-armored private key (`gpg --armor --export-secret-keys <id>`).
   - `PASSPHRASE`: the key's passphrase (omit if the key has none).
3. Publish the provider on the registry, pointing it at this repo.

Cutting a release (the tag's `x.y` must match the uapi `x.y` the release covers; see
[Versioning and compatibility](#versioning-and-compatibility)):

```sh
git tag v1.0.0
git push origin v1.0.0
```

The workflow then publishes a GitHub Release the registry can ingest. Validate
the config locally with `goreleaser check` and dry-run with
`goreleaser build --snapshot --single-target --clean`.

## License

MIT.
