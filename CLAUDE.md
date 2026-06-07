# terraform-provider-uapi

Terraform / OpenTofu provider for [uapi](https://github.com/openwrt-iac/uapi), the native HTTP REST API for OpenWrt. It manages OpenWrt configuration through uapi's curated endpoints.

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
main.go                       providerserver entry; address registry.terraform.io/openwrt-iac/uapi
internal/client/client.go     transport: bearer auth, 423/429 retry, idempotency key, cursor pagination, error envelope -> *APIError
internal/gen/                  code generator: openapi.json + descriptors.go overlay -> the <type>_*.go files below
internal/provider/
  provider.go                 schema (endpoint/token/insecure), Configure, registration (incl. ephemeral)
  helpers.go                  map<->tfsdk conversion (strVal/boolVal/int64Val/listVal, putStr/putBool/putInt64/putList), resolveImportID
  schema_helpers.go           shared attribute builders, diagsink
  ds_helpers.go               data-source attribute builders
  zz_generated_registration.go  (generated) the generated*() constructor slices
  <type>_resource.go          (generated) one CRUD resource per curated type, plus the system singleton
  <type>_data_source.go       (generated) lookup-by-id data sources
  token_ephemeral.go          uapi_token ephemeral resource (mint on Open, revoke on Close)
  whoami/healthz/diagnostics_data_source.go, dhcp_leases*_data_source.go, package*/authorized_key/system_password   hand-written specials (not generated)
```

Each resource is a typed model with `tfsdk` tags, a `body()` (model -> request map) and a `read()` (response map -> model). Data sources reuse the resource model and its `read()`.

## Code generation

The curated CRUD/singleton resources and their data sources are **generated** by `internal/gen`
(`make gen`, or `go generate ./...` which runs it before tfplugindocs). Inputs:

- `internal/gen/openapi.json`: a vendored copy of uapi's spec. Field **names, types, `readOnly`,
  `writeOnly`** are read from here, so strict-int and snake_case changes within a uapi minor are
  picked up automatically by re-vendoring the spec and re-running `make gen`.
- `internal/gen/descriptors.go`: the hand-maintained overlay supplying what the spec cannot express:
  required-ness, nested `match` structure, the `runtime` sub-shape, and Optional-vs-Optional+Computed.

The generator is **idempotent** (CI fails if committed output drifts from a fresh run) and must stay
that way: no `Date.now()`/random/map-iteration-order in emitted output. Do not hand-edit the
generated `<type>_resource.go`, `<type>_data_source.go`, `zz_generated_registration.go`, or the
generated `examples/` snippets; change `descriptors.go` (or the spec) and regenerate. The
hand-written specials are skipped by the generator and edited directly.

## Conventions that prevent bugs

- **Server-defaulted fields are `Optional + Computed`** with `UseStateForUnknown` (see the `optionalComputed*` helpers). Omitting them must not produce a perpetual diff against the value uapi fills in. New fields that the API defaults or normalizes follow this pattern; only genuinely caller-owned fields are plain `Required`/`Optional`.
- **`body()` sends only known, non-null attributes.** Unknown computed values are omitted so the server applies its default. The `putX` helpers enforce this.
- **Updates use `PUT` (full replace),** matching Terraform's "plan is the complete desired state". `PATCH` (merge) is only for the `system` singleton.
- **`id` is the uapi ULID,** computed with `UseStateForUnknown`.

## Forward compatibility (the uapi v1 contract)

uapi keeps `/api/v1/` additive: it may add response fields, optional request fields, resources,
error codes, and enum values within v1, and only bumps to `/api/v2/` for breaking changes. The
provider depends on that, so preserve these invariants:

- **Read responses into a map and pick known keys.** Never switch to strict struct decoding that
  errors on unknown fields; uapi adds response fields within v1 and they must be ignored.
- **No client-side enum validation.** `target`, `proto`, `encryption`, and friends are plain
  strings validated server-side, so uapi can add allowed values within v1 without a provider
  release. Schema descriptions list current values for humans only; do not add enum validators.
- **Branch on HTTP status, not the error `code` string.** `code`/`message` are surfaced in
  diagnostics, but control flow (e.g. 404 -> RemoveResource) keys off the status only.
- **Keep the client API-version-agnostic.** The major version lives in the user-supplied
  `endpoint` path; do not hardcode `/api/v1`.

**Version numbering:** the provider mirrors uapi. Provider `x.y.*` covers exactly the surface of
uapi `x.y.*` (major and minor track uapi; patch is the provider's own bugfix line). Tag releases
accordingly: a release covering uapi `1.2` is `v1.2.<patch>`. When uapi ships a new minor with new
resources or fields, the matching provider minor adds them; a breaking uapi major maps to a
provider major.

## Resource-specific gotchas

- **`uapi_system` is a singleton.** No id segment, no create/delete on the wire. Create and Update both `PATCH /system`; Delete is a no-op that only drops state.
- **`uapi_wireless_interface.key` is write-only.** uapi never returns it, so `read()` must leave the model's key untouched (preserving the planned value) and rely on the computed `has_key`. Do not map a key field out of the response.
- **Import adopts.** `importByID` (in `helpers.go`) wraps `resolveImportID`, which checks `managed`; an unmanaged (anonymous) section is adopted via `POST .../adopt`, which renames it and changes its id. Import is therefore a mutating operation for unmanaged sections (intentional), and `importByID` emits a warning diagnostic naming the old and new ids when it adopts. All resource `ImportState` methods are one-liners delegating to `importByID`.
- **423 locked is retried in the client,** honoring `Retry-After`. Do not add retry logic in resources.

## Testing

- `make test` runs unit tests: client behavior against `httptest` (423 retry, error envelope, 404, list) and the value-conversion helpers.
- `TestProviderSchema` drives the protocol server's `GetProviderSchema`, which validates every resource and data source schema at once. Run it after any schema change; it catches `Required`+`Computed` conflicts and nested-type mistakes.
- `make testacc` runs the `terraform-plugin-testing` acceptance suite (`TestAcc*`): it serves the provider in-process and drives a real terraform binary against an **in-process fake uapi** (`mock_uapi_test.go`), so it needs no router and runs in CI (the `acceptance` job). The fake is JSON-native and covers the curated CRUD/adopt/ETag/singleton/specials surface; extend it when a new pattern needs coverage. The suite tests *patterns* (flat CRUD + import, nested match, write-only secret, singleton, import-adopt, list + runtime data sources, the 1.2 resources), not every resource one by one.
- Quick end-to-end without acceptance tests: `make install`, point `TF_CLI_CONFIG_FILE` at `examples/dev.tfrc`, then `terraform validate` / `plan` in `examples/`.

## Docs

`docs/` is generated by `tfplugindocs` (`make docs` / `go generate ./...`) from the schema
attribute `Description`s and the snippets under `examples/` (`examples/provider/provider.tf`,
`examples/resources/<type>/{resource.tf,import.sh}`, `examples/data-sources/<type>/data-source.tf`).
Never hand-edit `docs/`; change the schema description or the example and regenerate. Give every new
attribute a `Description` (resources and data sources both) or its doc row will be blank. Commit the
regenerated `docs/` alongside the code change.

**Narrative guides** (the "Guides" section on the registry, e.g. the v1->v2 migration note) live in
`templates/guides/<name>.md.tmpl` and are rendered to `docs/guides/<name>.md` by tfplugindocs. Do
NOT write them directly into `docs/guides/`: tfplugindocs deletes any guide file that has no
`templates/guides/` source on the next `make docs`. Edit the `.md.tmpl` and regenerate.

## Source of truth for the API

The uapi contract is `build/openapi.json` in the uapi repo, with field defaults visible in its `src/resources/*.uc` (`fromUci`/`validate`) and write semantics in `src/lib/handler.uc`. When adding a resource, read those before writing the schema.
