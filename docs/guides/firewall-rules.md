---
page_title: "Firewall rules, redirects, and forwardings"
subcategory: "Guides"
description: |-
  Worked examples for the firewall resources, and the flat-vs-nested match distinction that is easy to get wrong.
---

# Firewall rules, redirects, and forwardings

The firewall resources split selectors two different ways, which is the single
most common authoring mistake:

- `uapi_firewall_rule` and `uapi_firewall_redirect` put their selectors in a
  **nested `match = { ... }` object**, where `proto`/`src_port`/`dest_port`/
  `dest_ip`/`src_dport` are **lists**.
- `uapi_firewall_forwarding` is **flat** (`src`/`dest` zone names, no `match`).

If you write a rule with flat `src`/`proto`/`dest_port` (like a forwarding), it
will not validate. Use the nested form below.

## Allow rule (nested match)

Allow inbound TCP 22 from the `lan` zone:

```hcl
resource "uapi_firewall_rule" "allow_ssh" {
  target = "ACCEPT"
  match = {
    src_zone  = "lan"     # required; an existing zone name (or a managed zone's id)
    proto     = ["tcp"]   # list
    dest_port = ["22"]    # list
  }
}
```

## Port forward / DNAT (nested match)

Forward `wan` TCP 8443 to an internal host on 443:

```hcl
resource "uapi_firewall_redirect" "https" {
  target = "DNAT"
  match = {
    src_zone  = "wan"
    proto     = ["tcp"]
    src_dport = ["8443"]            # incoming (destination) port to redirect
    dest_ip   = ["192.168.1.50"]    # internal target
    dest_port = ["443"]             # internal port
  }
}
```

## Zone forwarding (flat)

Allow traffic from `lan` to a `guest` zone. Note: **no `match` block** here, the
fields are top-level:

```hcl
resource "uapi_firewall_forwarding" "lan_to_guest" {
  src  = "lan"
  dest = "guest"
}
```

## Zone references

`match.src_zone` / `match.dest_zone` and forwarding `src`/`dest` take a **zone
name**. For a pre-existing zone (`lan`, `wan`) use the name directly; for a zone
you manage with Terraform, use its `id` (a managed section's name is its ULID).
See the "Referencing other resources" guide for the full convention.
