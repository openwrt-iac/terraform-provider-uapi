---
page_title: "Migrating from v1.x to v2.0"
subcategory: "Guides"
description: |-
  How to move a configuration from terraform-provider-uapi v1.x (uapi 1.x) to v2.0 (uapi 2.0).
---

# Migrating from v1.x to v2.0

Provider `v2.0` tracks uapi `2.0`, a breaking major. The set of managed
resources is unchanged, but the wire contract changed in ways that require you to
update configuration and, for a few fields, re-import existing state.

There are **no automatic state upgraders**. v2.0 is a clean major: you point the
provider at the new API version, adjust the configuration changes below, and let
Terraform reconcile. Where a stored attribute's type changed, refreshing or
re-importing the resource resolves it.

## 1. Point the endpoint at `/api/v2`

uapi serves v2 under a new path prefix. Update the provider (or
`UAPI_ENDPOINT`):

```hcl
provider "uapi" {
  endpoint = "https://router.example.com/api/v2" # was /api/v1
}
```

The client itself is version-agnostic; the version lives entirely in the
endpoint path.

## 2. Integer fields are now numbers, not strings

uapi 2.0 returns and accepts real JSON integers for uci integer fields. The
provider models these as `Number` instead of `String`. Quoted values that HCL
can coerce keep working, but the idiomatic form is now unquoted:

```hcl
resource "uapi_network_bridge_vlan" "v" {
  device = "br-lan"
  vlan   = 9 # was "9"
}

resource "uapi_dropbear_instance" "ssh" {
  port = 22 # was "22"
}
```

Affected fields include `vlan`, `port`, `lookup`, and the other uci integer
options across dropbear, uhttpd, sqm, snmpd, dhcp, mwan3, and network resources.
After upgrading, run `terraform plan`; a refresh reconciles the stored string to
the new number type with no resource replacement.

## 3. snake_case field renames

Several resources renamed fields to snake_case to match uapi 2.0:

- `uapi_dropbear_instance`, `uapi_snmpd_system`, `uapi_vnstat_config` (the 1.x to
  2.0 renames)
- `uapi_firewall_zone` / `uapi_firewall_defaults`: `output` is now `output_policy`
- `uapi_unbound_server`: `resource` is now `resource_limits`
- `uapi_mwan3_interface`: the uci `count` option is exposed as `probe_count`

Update any references to the renamed attributes (see each resource's doc page for
the current names) and re-run `terraform plan`.

## 4. New surface in 2.0

v2.0 adds a short-lived credential and read-only operational data sources:

- `uapi_token` (ephemeral resource): mints a scoped bearer token for the duration
  of the run and revokes it afterward. The cleartext token is never written to
  state. Requires Terraform 1.10+ or OpenTofu 1.11+.
- `uapi_whoami`, `uapi_healthz`, `uapi_diagnostics`: read-only operational data
  sources.

It also adds curated resources for `mwan3`, `usteer`, and `openvpn` where those
daemons are installed.

## 5. Transport behavior

The client now retries `429 Too Many Requests` (honoring `Retry-After`, like the
existing `423 Locked` retry), follows cursor pagination on collection reads, and
sends an `Idempotency-Key` on every `POST` so a retried create cannot duplicate a
resource. No configuration change is required.

## 6. Minimum uapi version

Use uapi `2.0.1` or newer. `2.0.0` computed per-resource `ETag`s over
package-wide state, which could surface a spurious `412 precondition_failed`
during a concurrent `destroy` of several resources in the same uci package;
`2.0.1` makes the ETag a per-resource body hash and resolves it.
