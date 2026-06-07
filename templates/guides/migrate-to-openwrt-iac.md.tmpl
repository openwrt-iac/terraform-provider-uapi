---
page_title: "Provider moved to the openwrt-iac namespace"
subcategory: "Guides"
description: |-
  The provider source address changed from raspbeguy/uapi to openwrt-iac/uapi. How to update an existing configuration.
---

# Provider moved to the openwrt-iac namespace

Starting with **v2.1.0**, the provider lives under the `openwrt-iac` org (the same
move uapi itself made). Its registry source address changed:

- old: `raspbeguy/uapi`
- new: `openwrt-iac/uapi`

The schema, resource set, and the ULID `id`s are unchanged, and nothing on the router
changes. But because the **source address** changed, existing state still binds every
resource to the old provider, so one extra command (`state replace-provider`) is
required. There are no schema state upgraders to run.

## What to change

1. Update `required_providers.source` in every root and child module:

   ```hcl
   terraform {
     required_providers {
       uapi = {
         source  = "openwrt-iac/uapi" # was "raspbeguy/uapi"
         version = ">= 2.1.0"
       }
     }
   }
   ```

2. Re-initialize so the new provider is downloaded and the lock file is rewritten:

   ```console
   $ terraform init -upgrade
   ```

3. Repoint existing **state** from the old provider address to the new one. This is the
   step that is easy to miss: without it `terraform plan` fails with "Provider
   configuration not present ... registry.terraform.io/raspbeguy/uapi ... has been
   removed", because state still references the old source.

   ```console
   $ terraform state replace-provider registry.terraform.io/raspbeguy/uapi registry.terraform.io/openwrt-iac/uapi
   ```

After those three steps, `terraform plan` shows **no changes** for resources that were
already applied: the provider is the same code under a new name.

OpenTofu users: identical, using `tofu init -upgrade` and `tofu state replace-provider`.
Until the provider is on the OpenTofu registry, point a `dev_overrides` / mirror at
`openwrt-iac/uapi` (see `examples/dev.tfrc`).

## Why

uapi moved from `github.com/raspbeguy/uapi` to `github.com/openwrt-iac/uapi`, with its
feed and site on `openwrt-iac.github.io`. The provider follows it so both live in one
place. v2.1.0 also adds the `uapi_unbound_srv` and `uapi_unbound_ext` resources that
ship in uapi 2.1.0, hence the `>= 2.1.0` requirement.
