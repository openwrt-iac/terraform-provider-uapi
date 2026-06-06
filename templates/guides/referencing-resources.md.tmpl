---
page_title: "Referencing other resources (managed sections use their id)"
subcategory: "Guides"
description: |-
  How to wire one uapi resource to another, and why managed sections are referenced by id rather than by a human name.
---

# Referencing other resources

Many uapi resources reference another section by name: `uapi_dhcp_server.interface`
points at a network interface, `uapi_network_bridge_vlan.device` at a device,
`uapi_firewall_rule.match.src_zone` at a firewall zone, `uapi_snmpd_access.group`
at an snmpd group, and so on.

The thing to know: **a uapi-managed section's name is the ULID uapi assigned it**
(its `id`). So when the section you are referencing is itself managed by Terraform,
reference it by `.id`, not by a label you chose:

```hcl
resource "uapi_network_interface" "lab" {
  proto   = "static"
  ipaddr  = "192.168.250.1"
  netmask = "255.255.255.0"
}

resource "uapi_dhcp_server" "lab" {
  # the interface's name IS its ULID, so use .id
  interface = uapi_network_interface.lab.id
}
```

Using a hand-picked string here (`interface = "lab"`) fails server-side with
`422 ... interface "lab" does not exist`, because no section by that name exists:
uapi named it with a ULID.

## Pre-existing (unmanaged) sections

Sections created outside Terraform (by LuCI, SSH, `wifi config`, or shipped in the
default config) keep their human name and are referenced by it. Common examples are
the stock interfaces and zones and the radios that `wifi config` generates:

```hcl
resource "uapi_firewall_rule" "ssh" {
  target = "ACCEPT"
  match = {
    src_zone  = "lan" # pre-existing zone, referenced by name
    proto     = ["tcp"]
    dest_port = ["22"]
  }
}

resource "uapi_wireless_interface" "ap" {
  device = "radio0" # pre-existing radio, referenced by name
  ssid   = "example"
}
```

If you later `terraform import` one of those unmanaged sections, uapi **adopts** it
and renames it to a ULID (import is a mutating operation for unmanaged sections; the
provider emits a warning naming the old and new id). After adoption, reference it by
its new `id` like any other managed section.

## Exception: references to a *device* use its name, not its id

A `uapi_network_device` is named by its own `name` field (the kernel device name,
e.g. `br-lan`), while its `id` is still a ULID. Resources that point at a *device*
reference that **name**, not the `.id`:

```hcl
resource "uapi_network_device" "br" {
  name  = "br-tf"
  type  = "bridge"
  ports = ["lan1"]
}

resource "uapi_network_bridge_vlan" "v" {
  device = uapi_network_device.br.name # the device NAME, not .id
  vlan   = 9
}
```

The same holds for any field that names a kernel object directly rather than a uapi
section: a network interface's `device`, and `uapi_sqm_queue.interface` /
`uapi_vnstat_interface.interface` (a device/interface name). Rule of thumb: if the
field is a *section* reference, use `.id`; if it names a kernel device, use the
device's `name` (or the literal kernel name for pre-existing devices).

## Destroy semantics

`terraform destroy` of an adopted or imported resource **deletes** it, the same as
a resource Terraform created (the standard Terraform contract). To stop managing a
section *without* deleting it, use `terraform state rm <address>` instead of
`destroy`. Singletons (`uapi_system`, `uapi_dhcp_dnsmasq`, `uapi_unbound_server`,
etc.) cannot be deleted: their `destroy` drops them from state and leaves the
router's settings as last applied. `uapi_package` will not uninstall a package that
was already installed before Terraform managed it (`pre_existed = true`).
