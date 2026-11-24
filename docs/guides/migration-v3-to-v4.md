---
page_title: "Migrating from provider 3.x to 4.0"
subcategory: "Guides"
description: |-
  What changes moving from uapi 3.x to 4.0, and the order to do it in.
---

# Migrating from provider 3.x to 4.0

Provider `4.*` targets uapi `4.*`. A uapi installation serves exactly one API
major, so the router moves to uapi 4.0 and the provider moves with it, in the
same maintenance window.

uapi 4.x runs on OpenWrt main and on the next OpenWrt release cut from it. It
does not install on OpenWrt 25.12. A router that stays on 25.12 stays on uapi
3.5.0, and provider 3.5 keeps managing it.

The uapi project lists every change of the wire surface in
[docs/migration-v3-to-v4.md](https://github.com/openwrt-iac/uapi/blob/v4.0.0/docs/migration-v3-to-v4.md).
This guide covers what a Terraform user does about them.

Do it in this order. Steps 1 and 2 are on the old versions.

## 1. Before upgrading anything, prepare on provider 3.5 and uapi 3.5

**Take each `uapi_openvpn_instance` out of state.** uapi 4.0 removes
`openvpn/instances`, and provider 4.0 removes the resource. On OpenWrt main, an
OpenVPN tunnel is a `uapi_network_interface` with `proto = "openvpn"`. Remove the
instances from state now, while the provider still knows the resource type. This
keeps the configuration on the router:

```hcl
removed {
  from = uapi_openvpn_instance.office

  lifecycle {
    destroy = false
  }
}
```

A `removed` block needs Terraform 1.7 or later. `terraform state rm
uapi_openvpn_instance.office` does the same.

**Set the tunnels that must not run, and the clients that must not take the
default route.** At the first boot on OpenWrt main, the openvpn package copies
each `config openvpn` section into `/etc/config/network` once, under the same
name. Two things then change, and both happen before you can apply anything:

- netifd starts every copied tunnel, also an instance that was off on 3.x.
- A client takes the default route through the tunnel each time openvpn knows a
  route gateway, and the main table has no IPv4 default route after the tunnel
  goes down, until wan restarts.

Before the upgrade, add these options on the router, in `/etc/config/openvpn`.
The copy carries them, and OpenWrt 25.12 does not read them:

```sh
uci set openvpn.office.disabled='1'      # for each instance that is off
uci set openvpn.office.defaultroute='0'  # for each client that must not carry the default route
uci commit openvpn
```

**Fix the values that uapi 4.0 refuses.** uapi 4.0 answers 422 for a write of
each of these. Provider 3.5 can write the fix:

- `uapi_mwan3_globals.loglevel`: change `err` to `error` and `warn` to `warning`.
- `uapi_mwan3_member.metric`: set a value from 1 to 256.
- `uapi_firewall_zone.name`: a name must start with a letter or an underscore.
  Change the zone name in the rules, redirects and forwardings that name it too.
- `uapi_network_rule.ipproto`: write a number from 1 to 255, such as `6`, in
  place of a name such as `tcp`.
- A string attribute that holds a newline or a carriage return. Only
  `uapi_system.notes` keeps several lines.
- `uapi_token.scopes`: remove `dhcp:leases6` and `openvpn:instances`, and a scope
  on a package that is not installed. A token that reads the IPv6 leases needs
  `dhcp:leases` in uapi 4.0.

**Find the stored errors.** uapi 4.0 refuses every write of a section that holds
a value that its daemon refuses, also when the write does not change that value.
The validation sweep of uapi 3.5 lists most of them:

```hcl
data "uapi_diagnostics" "preflight" {
  validate = true
}

output "invalid_sections" {
  value = data.uapi_diagnostics.preflight.invalid_sections
}
```

Fix each section that it lists. The sweep of 3.5 does not list a newline that
3.x took, and it lists some values that uapi 4.0 takes.

## 2. Confirm a clean plan, then snapshot state

```sh
terraform plan   # expect: no changes
```

Back up state before you continue.

## 3. Upgrade the router to OpenWrt main and uapi 4.0

Move the router to OpenWrt main, or to the next release cut from it, then install
uapi 4.0. The API moves to `/api/v4`, and provider 3.x cannot talk to it. The
time between this step and the next is the outage.

## 4. Point the provider at `/api/v4` and upgrade it

```hcl
provider "uapi" {
  endpoint = "https://192.168.1.1/api/v4" # was /api/v3
}

terraform {
  required_providers {
    uapi = {
      source  = "openwrt-iac/uapi"
      version = "~> 4.0"
    }
  }
}
```

Then edit the configuration for the attributes whose type or behavior changed.

**`uapi_wireless_interface.device` and `network` are lists.**

```hcl
resource "uapi_wireless_interface" "home" {
  device = ["radio0"] # was: device = "radio0"
  network = ["lan"]   # was: network = "lan"
}
```

The state migrates on the first plan, and the router keeps one radio stored as a
string.

**`uapi_firewall_defaults.synflood_rate` is a string with its whole unit.** Write
`"25/second"`, not `25`. A number becomes the string `"25"`, which uapi refuses.

**A zone policy that the configuration does not set is removed.**
`uapi_firewall_zone` `input`, `output_policy` and `forward` follow the policy of
`uapi_firewall_defaults` when unset, and they read null then. Removing one from
the configuration now removes the policy from the router. Set each policy that a
zone must keep.

**Set the sqm options that a queue must keep.** `uapi_sqm_queue` now models the
options of LuCI's sqm form. `ilimit`, `elimit`, `itarget`, `etarget`, `iqdisc_opts` and
`eqdisc_opts` follow the same rule as the zone policies: removing one from the
configuration removes it from the router. A queue that LuCI made holds `itarget = "auto"`
and `etarget = "auto"`, so set both, or the first apply after an import removes them. The
plan shows each such removal.

**A new access point needs `mode = "ap"`.** uapi 4.0 stores no mode when the
configuration sets none, and wifi-scripts sets up nothing for a section without
a mode.

**Import the OpenVPN tunnels as `uapi_network_interface`.** Write each tunnel
with `proto = "openvpn"`, then import it by its id. Do not let Terraform create
it: uapi answers 422 for a create with an id that `/etc/config/network` holds.

```hcl
resource "uapi_network_interface" "office" {
  id           = "office"
  proto        = "openvpn"
  client       = true
  dev          = "tun0"
  ovpnproto    = "udp"
  remote       = ["vpn.example.com 1194"]
  ca           = "/etc/openvpn/office/ca.crt"
  key          = "/etc/openvpn/office/client.key"
  defaultroute = false
}

import {
  to = uapi_network_interface.office
  id = "office"
}
```

The attributes of `uapi_openvpn_instance` keep their names, with these changes:

| Provider 3.x `uapi_openvpn_instance` | Provider 4.0 `uapi_network_interface` |
|---|---|
| `proto` | `ovpnproto` |
| `enabled` | `disabled`, inverted |
| `status`, `comp_lzo`, `compress`, `ncp_ciphers`, `ncp_disable` | gone: the handler of OpenWrt main does not take them |
| (none) | `config`, `data_ciphers`, `username`, and the write-only `password` and `cert_password` |

Set `data_ciphers` to one entry, the cipher names joined by colons. A boolean
reads true only for a stored `1` or `true`, and a port also reads in hex or
octal, so the first plan after the import can show a change for such a value.
The tunnel needs `openvpn-openssl`, or its `openvpn-mbedtls` or `openvpn-wolfssl`
variant. netifd loads the handler of a package installed later only after a
reboot or `/etc/init.d/network restart`.

**Set the login of a pppoe, ppp or wwan interface.** uapi 4.0 models `username`
and the write-only `password` on these protos. A switch to another proto removes
both, unless the configuration sets them. `password = ""` removes the stored
password, and removing `password` from the configuration keeps it.

```sh
terraform init -upgrade
terraform plan
```

## What to expect on that first plan

**Some reads change.** uapi 4.0 reads several options as their daemon reads them,
such as `uapi_system.log_remote`, the dropbear flags, `uapi_sqm_queue.enabled`,
the zone policies and `uapi_firewall_defaults.syn_flood`. The plan refreshes
first. An attribute that the configuration does not set takes the new read, and
the apply writes nothing for it, or sends the new read back. An attribute that
the configuration sets writes its value, and the daemon then does what the
configuration says, which it did not do before. Read each such change in the plan
before you apply it.

**A firewall bool can plan as known after apply.** A stored value that firewall4
cannot parse, such as an `enabled` of `YES`, reads null in uapi 4.0, and firewall4
skips the section. When the configuration does not set the attribute, the next
apply removes that value, and the attribute reads its default.

**A write can fail for a value that it does not change.** uapi 4.0 refuses every
write of a section that holds a value that its daemon refuses. The 422 names the
attribute. Set a valid value in the configuration, and the same apply fixes the
section.

**`has_*` attributes read false outside their proto.** The write-only secrets of
`uapi_network_interface` report `has_key`, `has_password` and the others as false
on a proto that does not take them.

**A destroy of the `main` uhttpd instance fails.** uapi 4.0 answers 422 for a
delete of `uapi_uhttpd_instance` `main` while it holds the mount of uapi. Take it
out of state with a `removed` block instead.

**No resource is recreated by the upgrade itself.** The ids do not change. If the
plan proposes to destroy something, stop and find out why before you apply.

## Rolling back

uapi 3.x does not run on OpenWrt main. To roll back, put the router back on
OpenWrt 25.12 with uapi 3.5.0, pin the provider to `~> 3.5`, and restore the
state snapshot of step 2.
