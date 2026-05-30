# terraform-provider-uapi

A Terraform / OpenTofu provider for [uapi](../uapi), the native HTTP REST API for OpenWrt.
It manages OpenWrt configuration (firewall, network, wireless, DHCP, system) through uapi's
curated endpoints, which expose stable resource IDs and atomic, transactional writes.

## Scope

This provider targets the **curated** uapi endpoints only. It deliberately does **not** use the
`/raw/<package>/<id>` passthrough: raw payloads follow uci's field names directly and carry no
stability promise across OpenWrt releases, which is a poor fit for managed Terraform state.

## Requirements

- An OpenWrt router running uapi (OpenWrt 25.12+), reachable over HTTP(S).
- A bearer token created on the router: `uapi-token create --name terraform --scope '*:rw'`.
- Terraform >= 1.0 or OpenTofu.

## Provider configuration

```hcl
provider "uapi" {
  endpoint = "https://192.168.1.1/api/v1" # or env UAPI_ENDPOINT / UAPI_BASE
  token    = var.uapi_token               # or env UAPI_TOKEN
  insecure = true                         # or env UAPI_INSECURE=1
}
```

| Argument   | Env                          | Description                                                        |
|------------|------------------------------|--------------------------------------------------------------------|
| `endpoint` | `UAPI_ENDPOINT`, `UAPI_BASE` | API root including the `/api/v1` prefix.                            |
| `token`    | `UAPI_TOKEN`                 | Bearer token. Sensitive.                                           |
| `insecure` | `UAPI_INSECURE`              | Skip TLS verification. Needed for uapi's default self-signed cert. |

> **TLS:** uapi ships a self-signed certificate. `insecure = true` (or the marker file
> `/etc/uapi.insecure` on the router) gets you going quickly; for production, install a real
> certificate via `acme.sh` / `luci-app-acme` and leave `insecure` off.

## Resources

| Resource                   | uci backing            | Notes                                               |
|----------------------------|------------------------|-----------------------------------------------------|
| `uapi_firewall_rule`       | `firewall.rule`        | `target`, nested `match` block.                     |
| `uapi_firewall_zone`       | `firewall.zone`        | Input/output/forward policies, masquerading.        |
| `uapi_firewall_redirect`   | `firewall.redirect`    | Port forwards (DNAT/SNAT).                           |
| `uapi_network_interface`   | `network.interface`    | See the management-link caution below.              |
| `uapi_network_device`      | `network.device`       | Bridges, VLANs, etc.                                |
| `uapi_wireless_device`     | `wireless.wifi-device` | Radios.                                             |
| `uapi_wireless_interface`  | `wireless.wifi-iface`  | SSIDs. `key` is write-only; `has_key` is computed.  |
| `uapi_dhcp_host`           | `dhcp.host`            | Static leases.                                      |
| `uapi_system`              | `system.system`        | Singleton; see below.                               |

## Data sources

One lookup-by-`id` data source per resource type (`uapi_firewall_rule`, `uapi_firewall_zone`, ...,
`uapi_dhcp_host`), plus:

- `uapi_system`: the global system settings (no `id`).
- `uapi_dhcp_leases`: the current active DHCP leases reported at runtime (read-only).

## Behaviour notes

- **Stable IDs.** uapi assigns every managed section a prefixed ULID (e.g. `r_01HX...`) that
  survives reorders and rewrites; that ID is the Terraform resource `id`.
- **Server-defaulted fields** (booleans, enum fallbacks, etc.) are modeled as
  `Optional + Computed`, so omitting them in config does not produce perpetual diffs.
- **423 locked retries.** uapi serializes writes behind a global lock and returns `423` with
  `Retry-After` under contention. The provider retries automatically.
- **Write-only key.** `uapi_wireless_interface.key` is never returned by the API. The provider
  keeps the configured value in state and exposes `has_key` to tell whether one is set.

### `uapi_system` is a singleton

It cannot be created or destroyed. `terraform apply` writes the settings via `PATCH`; `terraform
destroy` only drops it from state and leaves the router's settings untouched.

### Importing adopts unmanaged sections

Pre-existing anonymous uci sections (created by LuCI, SSH, etc.) surface as `managed = false`.
Running `terraform import` on such a section **adopts** it: uapi renames it to a stable ULID and
flips it to managed. This means import is a *mutating* operation for unmanaged sections.

```sh
terraform import uapi_firewall_rule.example r_01HX...   # already-managed: read-only import
terraform import uapi_firewall_rule.example cfg0a1b2c   # anonymous: adopted, id becomes a ULID
```

> ⚠️ Be careful importing/editing `uapi_network_interface` for the interface that backs your
> management connection. uapi only observes the init script's exit code, not runtime convergence,
> so a bad change can lock you out.

## Building and local development

```sh
make build            # build ./terraform-provider-uapi
make install          # install into ~/.terraform.d/plugins for dev_overrides
make test             # unit tests
make fmt vet          # format and vet
```

To try it against a router, use a CLI config with `dev_overrides` (see `examples/dev.tfrc`):

```sh
make install
export TF_CLI_CONFIG_FILE=$PWD/examples/dev.tfrc   # edit the path inside first
cd examples
export UAPI_ENDPOINT=https://192.168.1.1/api/v1 UAPI_TOKEN=... UAPI_INSECURE=1
terraform plan
```

Acceptance tests (`make testacc`) require a live uapi instance and `TF_ACC=1`.

## License

MIT.
