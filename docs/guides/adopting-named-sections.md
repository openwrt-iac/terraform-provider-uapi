---
page_title: "Managing pre-existing named sections (lan, wan, br-lan)"
subcategory: "Guides"
description: |-
  How to safely manage a box's own named sections with the settable id and adopt-keep-name behaviour added in uapi 2.2.0.
---

# Managing pre-existing named sections

Every OpenWrt box ships named sections you often want to manage from Terraform:
`config interface 'lan' / 'wan' / 'wan6'`, the `br-lan` device, the stock
firewall zones. The provider gives every collection resource a **settable `id`**
(uapi >= 2.2.0): set it to choose the uci section name, or omit it to let uapi
assign a prefixed ULID.

```hcl
resource "uapi_network_interface" "guest" {
  id      = "guest" # the uci section name; also the resource id
  proto   = "static"
  ipaddr  = "192.168.5.1"
  netmask = "255.255.255.0"
}
```

`id` is **create-only**: changing it forces replacement (a uci rename is a new
section), and it is never sent on update.

## Adopting the box's own lan / wan

You cannot *create* a section whose name already exists - that returns `422`
(a `validation_failed` with a `conflict` field error; the provider hints to import
it). Instead, **import** it. Since uapi 2.2.0, a
named section is adopted **in place**: its name is kept (no rename to a ULID),
the import does not mutate the router, and config with the same `id` reconciles
with no replacement.

```hcl
resource "uapi_network_interface" "lan" {
  id    = "lan"
  proto = "static"
  # ...the rest of lan's settings...
}
```

```console
$ terraform import uapi_network_interface.lan lan
$ terraform plan   # No changes. lan keeps its name.
```

This is the safe path for the management interface: no destroy, no lockout. (On
older uapi, import renamed `lan` to a ULID and the next plan wanted to destroy +
recreate it - avoid managing your own `lan`/`wan` against pre-2.2.0 uapi.)

Anonymous sections (uci `cfgXXXXXX`, e.g. a zone created in LuCI without a name)
are still adopted by renaming to a stable id; the import emits a warning naming
the old and new id, because that case does mutate the router.

## Not just interfaces: zones, dhcp servers, sqm queues

A stock box also ships named `firewall_zone` (`lan`/`wan`), `dhcp_server`
(`lan`/`wan`), and `sqm_queue` sections. Manage these the same way: **import the
default first**, do not create a second one with the same name. On uapi >= 2.2.1 a
create whose name collides with an existing section is rejected (`422`, a
`validation_failed` whose field error code is `conflict`), rather than silently
producing a duplicate the firewall/dnsmasq can't disambiguate, and the provider's
error points you to import it. (Earlier uapi created the duplicate.)

```console
$ terraform import uapi_firewall_zone.lan lan
$ terraform import uapi_dhcp_server.lan lan
```

## Clearing leftover fields after adoption

An adopted interface can carry options from a prior life, e.g. a `wan` switched
from `proto=static` to `proto=dhcp` still has a stale `netmask`/`gateway` in uci.
Since uapi >= 2.2.3 those two fields are clear-on-omit: leave them out of config and
`terraform plan` shows them dropping to null (an in-place update, not a replace),
and apply clears the uci options. Fields uapi defaults or synthesizes are not
clear-on-omit, so they stay as last applied. (`ipaddr`/`ipaddrs`/`dns` are a known
gap, tracked upstream; only `netmask`/`gateway` clear by omission today.)

## Referencing an adopted section

A managed section's name is its `id`, so reference it by `.id` (see the
"Referencing other resources" guide). After adopting `lan`, other resources point
at `uapi_network_interface.lan.id` (which is just `"lan"`).
