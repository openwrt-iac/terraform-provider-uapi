---
page_title: "Firewall rules, NAT, redirects, and forwardings"
subcategory: "Guides"
description: |-
  Worked examples for the firewall resources, the three different match shapes, and the traps that make a router silently discard a rule.
---

# Firewall rules, NAT, redirects, and forwardings

The firewall resources spell their selectors three different ways, which is the
single most common authoring mistake:

| Resource | Selectors | Address / port types |
|---|---|---|
| `uapi_firewall_rule` | nested `match = { ... }` | **lists** |
| `uapi_firewall_redirect` | nested `match = { ... }` | **lists, but at most one value each** |
| `uapi_firewall_nat` | nested `match = { ... }` | **scalars** (only `proto` is a list) |
| `uapi_firewall_forwarding` | flat, no `match` | scalars |

The shapes are not an inconsistency in the provider: firewall4 parses each
section type differently, and the schema follows what it actually accepts. A
list where fw4 wants a scalar makes it drop the whole section.

## Allow rule (nested match, lists)

Allow inbound TCP 22 from the `lan` zone:

```hcl
resource "uapi_firewall_rule" "allow_ssh" {
  target = "ACCEPT"
  match = {
    src_zone  = "lan"     # optional; an existing zone name (or a managed zone's id)
    proto     = ["tcp"]   # list
    dest_port = ["22"]    # list
  }
}
```

`match.src_zone` is optional on a rule. Omit it for a rule about traffic the
router itself originates, which fw4 places in the `output` chain. Target
`NOTRACK` is the exception: it derives its chain name from the zone, so it
requires a real zone name and rejects both an absent `src_zone` and the `*`
wildcard.

## Matching a port means matching tcp or udp

firewall4 keeps a port match only on tcp and udp. For any other protocol it
drops the ports and emits the rule anyway, so the rule matches **more** traffic
than you asked for. With the `all` wildcard it matches everything:

```hcl
# Rejected with 422. Before uapi 2.4.0 this returned 200 and rendered a rule
# accepting all traffic, because fw4 emitted neither a protocol nor a port match.
resource "uapi_firewall_rule" "wrong" {
  target = "ACCEPT"
  match = {
    src_zone  = "wan"
    proto     = ["all"]
    dest_port = ["22"]
  }
}
```

Either name tcp/udp explicitly, or drop the port. A protocol list with no ports
is fine, as is omitting `proto` entirely (fw4 defaults it for itself).

## Marking and DSCP

`MARK` and `DSCP` targets need the value they set, or fw4 discards the section:

```hcl
resource "uapi_firewall_rule" "mark_voip" {
  target   = "MARK"
  set_mark = "0x40/0xff0"     # or set_xmark for XOR semantics
  match = {
    src_zone  = "lan"
    proto     = ["udp"]
    dest_port = ["5060"]
  }
}

resource "uapi_firewall_rule" "prioritize" {
  target   = "DSCP"
  set_dscp = "EF"             # class name or a value 0-63
  match = {
    src_zone = "lan"
    dscp     = "!EF"          # match options can negate with a leading "!"
    mark     = "0x40/0xff0"
  }
}
```

## Source NAT (nested match, scalars)

`uapi_firewall_nat` wraps `config nat`, the only way to express MASQUERADE or an
exemption from source NAT. Note the scalar addresses and ports:

```hcl
resource "uapi_firewall_nat" "masq_wan" {
  target = "MASQUERADE"
  match = {
    src_zone = "wan"            # the OUTBOUND (postrouting) zone
    src_ip   = "192.168.9.0/24" # scalar, not a list
  }
}

resource "uapi_firewall_nat" "snat_guests" {
  target  = "SNAT"
  snat_ip = "203.0.113.7"       # required by SNAT; snat_port is optional
  match = {
    src_zone = "wan"
    src_ip   = "192.168.20.0/24"
    proto    = ["tcp"]          # proto is still a list
  }
}
```

`match.family` is deliberately not defaulted. firewall4 reads an absent family
on a NAT section as IPv4 only, for backwards compatibility, so set
`family = "any"` explicitly if you want dual-stack.

To masquerade all egress traffic, write an explicit empty block, `match = {}`.
uapi accepts a NAT section with no match at all, but the provider's schema
requires the block, so the empty form is how you say "match everything".

## Port forward / DNAT (nested match, one value per list)

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

The match fields stay lists on the wire, but a redirect accepts **at most one
value** in each: fw4 treats them as scalars on a `config redirect` and drops any
section that writes a uci list. A second value is a `422`.

`match.src_dip` is the external destination address. On a DNAT it is matched
against, and it also selects the address used for NAT reflection. On an SNAT
redirect it is the address the source is rewritten to, and it is required:

```hcl
resource "uapi_firewall_redirect" "snat_legacy" {
  target = "SNAT"
  match = {
    src_zone  = "lan"
    dest_zone = "wan"               # required, and cannot be the "*" wildcard
    src_dip   = ["203.0.113.7"]     # required on SNAT
    proto     = ["tcp"]
  }
}
```

For new configuration prefer `uapi_firewall_nat` for source NAT. It is where
LuCI migrates these sections, and its scalar match is a closer fit to what fw4
parses.

## `target` is case-sensitive

`target` values are upper-case:

- rules: `ACCEPT` / `REJECT` / `DROP` / `NOTRACK` / `MARK` / `DSCP`
- redirects: `DNAT` / `SNAT`
- NAT: `SNAT` / `MASQUERADE` / `ACCEPT`

A lower-case value (`"dnat"`) is rejected server-side with `422 not_in_enum`.
The provider does not normalize case or validate the set (enums are validated by
uapi so new values work without a provider release), so write them as shown.

The `HELPER` target is deliberately not exposed. A rule naming a conntrack
helper whose kernel module is absent makes the **entire** nftables ruleset fail
to load, and the helper packages are not installed by default.

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

There is exactly one wildcard, `*`. The value `any` is not a synonym for it: it
resolves against real zone names, so a zone actually named `any` is the only
thing it matches.
