---
page_title: "Deprecations"
subcategory: "Guides"
description: |-
  Provider attributes that are deprecated but still accepted during a deprecation window.
---

# Deprecations

Attributes here still work but are scheduled for removal in a future major. The
provider mirrors uapi's deprecation policy: a deprecation lands in a minor (both
old and new forms accepted), removal happens no sooner than the next major.

## Active

| Attribute | Replaced by | Deprecated since | Removal target | Migration |
|---|---|---|---|---|
| `uapi_network_interface.name` | `uapi_network_interface.id` | provider 2.2.0 (uapi 2.2.0) | v3 | Set `id` instead of `name` at create. Both pick the uci section name and are accepted during the window; if you supply both they must match. `id` is the universal section-name input on every collection resource (see the "Managing pre-existing named sections" guide); `name` was a 2.1.0-era shim that only worked on `uapi_network_interface`. |

## Migrating `name` to `id`

You do not have to migrate during v2: `name` keeps working for the whole window.
When you do switch to `id`, set it to the value `name` had:

```hcl
resource "uapi_network_interface" "wg0" {
  # name = "wg0"   # was
  id          = "wg0" # now
  proto       = "wireguard"
  private_key = var.wg_key
}
```

Since provider 2.2.1 this is a **non-destructive in-place update**, not a
replacement: dropping `name` clears it from state while `id` carries the same
section name, so `terraform plan` shows an in-place update (not "must be
replaced") and the interface is never torn down. It is safe to do on the
interface that backs your management connection. Only a real rename replaces:
changing `id` to a new value, or changing `name` to a *different* value (provider
< 2.2.1 replaced on any `name` removal too).
