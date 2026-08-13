---
page_title: "Migrating from provider 2.x to 3.0"
subcategory: "Guides"
description: |-
  What breaks moving from uapi 2.x to 3.0, and the order to do it in.
---

# Migrating from provider 2.x to 3.0

Provider `3.*` targets uapi `3.*`. A uapi installation serves exactly one API
major, so this is not a gradual migration: the router moves to uapi 3.0 and the
provider moves with it, in the same maintenance window.

Do it in this order. Steps 1 and 2 are on the old version, and skipping them
turns a clean upgrade into a broken plan.

## 1. Before upgrading anything, fix your configuration on 2.x

Everything in this step is expressible in provider 2.5.x, so you can make these
edits, apply them, and confirm a clean plan while still on the old version.

**Replace `uapi_network_interface.name` with `id`.** The deprecated alias is gone
in 3.0. Both were accepted through 2.x, and if you set `name` the migration is a
rename in place, not a replacement:

```hcl
resource "uapi_network_interface" "lan" {
  id    = "lan"   # was: name = "lan"
  proto = "static"
}
```

**Replace `uapi_dhcp_host.mac` and `mac_aliases` with `macs`.** One list replaces
the scalar-plus-extras pair:

```hcl
resource "uapi_dhcp_host" "printer" {
  macs = ["02:00:00:00:00:41"]   # was: mac = "..." and mac_aliases = [...]
}
```

**Replace the `uapi_vnstat_interface` resources with `uapi_vnstat_config.interfaces`.**
The resource is gone in 3.0: `vnstat/interfaces` wrote a uci section vnstat never
reads. The list on the singleton is the setting the daemon actually uses, and it
takes kernel device names:

```hcl
resource "uapi_vnstat_config" "v" {
  interfaces = ["br-lan", "eth0"]
}
```

Remove the old resources from state rather than destroying them, since the
sections they point at are inert either way:

```sh
terraform state list | grep uapi_vnstat_interface | xargs -n1 terraform state rm
```

**Stop setting `uapi_network_interface.ipaddr`.** It is read-only in 3.0, a view of
the first `ipaddrs` entry. Write the list instead, even for one address:

```hcl
ipaddrs = ["192.168.1.1"]   # was: ipaddr = "192.168.1.1"
```

**Drop the attributes 3.0 removes outright.** These wrote uci options no OpenWrt
component reads, so removing them from your configuration changes nothing on the
router:

- all 18 collector toggles on `uapi_prometheus_node_exporter_lua_config`
  (`cpu`, `meminfo`, `netdev`, and the rest). The exporter enumerates its
  collectors from disk, and seven of the toggles named collectors the package does
  not even ship
- `uapi_vnstat_config.database_dir`, `interface_5min_hours`, `month_rotate`
- `uapi_mwan3_globals.local_source`, `rtmon_interval`
- `uapi_unbound_server.enabled`, `prefetch`
- `uapi_usteer_config.max_assoc_sta`
- `uapi_lldpd_config.enable_lldpmed`

Provider 2.5.0 warned about every one of these at plan time, so if you upgraded
through it you have already seen the list.

## 2. Confirm a clean plan, then snapshot state

```sh
terraform plan   # expect: no changes
```

Back up state before continuing. Nothing here rewrites state destructively, but
this is the point you would want to return to.

## 3. Upgrade the router to uapi 3.0

```sh
apk update && apk add --upgrade uapi
```

The API moves to `/api/v3` at the same time. Provider 2.x cannot talk to it, and
provider 3.x cannot talk to a 2.x router, so the window between this step and the
next is the outage.

## 4. Point the provider at `/api/v3` and upgrade it

```hcl
provider "uapi" {
  endpoint = "https://192.168.1.1/api/v3" # was /api/v2
}
```

```hcl
terraform {
  required_providers {
    uapi = {
      source  = "openwrt-iac/uapi"
      version = "~> 3.0"
    }
  }
}
```

```sh
terraform init -upgrade
terraform plan
```

Use **3.0.1 or later**. Two attributes changed their Terraform type, not just their
API shape: `uapi_firewall_redirect`'s match selectors became strings, and
`uapi_dhcp_host.tag` became a list back in 2.5.0. State written before those
changes is migrated automatically on the first plan.

Provider 3.0.0 itself shipped without those migrations, so a plan on 3.0.0 with
redirects in state fails to decode prior state and stops before showing a diff,
naming a schema mismatch rather than the attribute. If you are on 3.0.0 and stuck,
upgrading to 3.0.1 is the whole fix. Recovering without upgrading means
`terraform state rm` for **every** affected resource first and then importing them
back, because each import re-reads the entire state file: while any undecodable
entry remains, the import fails too, so removing and importing one at a time
reports failure for all but the last.

## What to expect on that first plan

**Lists you never set may move from `[]` to null.** uapi 3.0 answers null for an
absent uci list instead of an empty array, which is a real distinction: unset is
not the same as set-to-empty. The provider follows it, so a list attribute you
have never configured can show as changing from `[]` to null. That is state
catching up with what the router always meant, and it converges in one apply.

**`ipaddr` appears as a computed value.** It moves from something you could set to
something the router reports.

**No resource is recreated by the upgrade itself.** The ids are unchanged, and the
attribute removals are not replacement-forcing. If your plan proposes destroying
something, stop and work out why before applying.

## Rolling back

Reinstall the previous uapi package and pin the provider back to `~> 2.5`. Older
APKs stay attached to their
[uapi releases](https://github.com/openwrt-iac/uapi/releases), and the signed git
tag is the contract document for each. Configuration edited for 3.0 is mostly
valid on 2.x too, with one exception: `ipaddrs` for a single address works on both,
but a `uapi_vnstat_config.interfaces` list has no 2.x equivalent that reaches the
daemon.
