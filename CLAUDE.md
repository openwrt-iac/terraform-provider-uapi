# terraform-provider-uapi

Terraform / OpenTofu provider for [uapi](https://github.com/raspbeguy/uapi), the native HTTP REST API for OpenWrt. It manages OpenWrt configuration through uapi's curated endpoints.

## Code and documentation style

- **Priorities, in order:** simplicity, maintainability, modularity, readability.
- **No em-dashes.** Applies to code, comments, docs, and commit messages.
- **Comments are rare.** Default to none. Naming and structure carry the meaning.
- **When a comment is necessary, explain why, not what.** The non-obvious constraint or gotcha, not a restatement of the code.
- **Commit messages are one line.**

## Non-negotiable design decisions

1. **No `/raw`.** The provider only wraps uapi's curated endpoints. The `/raw/<package>/<id>` passthrough follows uci field names with no cross-release stability promise, which does not belong in managed Terraform state. Do not add resources or data sources backed by `/raw`.
2. **uapi is the source of truth.** After every write the provider reads the full response back into state, so server-side normalization (booleans, enum fallbacks, dropped empty lists) never drifts.
3. **The HTTP layer lives in one place.** `internal/client` owns auth, JSON, retry, and error decoding. Resources never build URLs by hand beyond the collection constant plus id, and never touch `net/http`.

## Architecture

```
main.go                       providerserver entry; address registry.terraform.io/raspbeguy/uapi
internal/client/client.go     transport: bearer auth, 423 retry, error envelope -> *APIError
internal/provider/
  provider.go                 schema (endpoint/token/insecure), Configure, registration
  helpers.go                  map<->tfsdk conversion (strVal/boolVal/listVal, putStr/putBool/putList), resolveImportID
  schema_helpers.go           shared attribute builders, diagsink
  <type>_resource.go          one CRUD resource per curated type, plus the system singleton
  <type>_data_sources.go      lookup-by-id data sources, plus dhcp_leases (list)
```

Each resource is a typed model with `tfsdk` tags, a `body()` (model -> request map) and a `read()` (response map -> model). Data sources reuse the resource model and its `read()`.

## Conventions that prevent bugs

- **Server-defaulted fields are `Optional + Computed`** with `UseStateForUnknown` (see the `optionalComputed*` helpers). Omitting them must not produce a perpetual diff against the value uapi fills in. New fields that the API defaults or normalizes follow this pattern; only genuinely caller-owned fields are plain `Required`/`Optional`.
- **`body()` sends only known, non-null attributes.** Unknown computed values are omitted so the server applies its default. The `putX` helpers enforce this.
- **Updates use `PUT` (full replace),** matching Terraform's "plan is the complete desired state". `PATCH` (merge) is only for the `system` singleton.
- **`id` is the uapi ULID,** computed with `UseStateForUnknown`.

## Resource-specific gotchas

- **`uapi_system` is a singleton.** No id segment, no create/delete on the wire. Create and Update both `PATCH /system`; Delete is a no-op that only drops state.
- **`uapi_wireless_interface.key` is write-only.** uapi never returns it, so `read()` must leave the model's key untouched (preserving the planned value) and rely on the computed `has_key`. Do not map a key field out of the response.
- **Import adopts.** `resolveImportID` (in `helpers.go`) checks `managed`; an unmanaged (anonymous) section is adopted via `POST .../adopt`, which renames it and changes its id. Import is therefore a mutating operation for unmanaged sections. This is intentional.
- **423 locked is retried in the client,** honoring `Retry-After`. Do not add retry logic in resources.

## Testing

- `make test` runs unit tests: client behavior against `httptest` (423 retry, error envelope, 404, list) and the value-conversion helpers.
- `TestProviderSchema` drives the protocol server's `GetProviderSchema`, which validates every resource and data source schema at once. Run it after any schema change; it catches `Required`+`Computed` conflicts and nested-type mistakes.
- `make testacc` (needs `TF_ACC=1` and a live uapi instance) is for acceptance tests against a real router.
- Quick end-to-end without acceptance tests: `make install`, point `TF_CLI_CONFIG_FILE` at `examples/dev.tfrc`, then `terraform validate` / `plan` in `examples/`.

## Source of truth for the API

The uapi contract is `build/openapi.json` in the uapi repo, with field defaults visible in its `src/resources/*.uc` (`fromUci`/`validate`) and write semantics in `src/lib/handler.uc`. When adding a resource, read those before writing the schema.
