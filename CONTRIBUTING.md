# Contributing

## Build and test

```sh
make build      # compile the provider
make test       # unit tests (client + helpers + schema validation)
make testacc    # acceptance suite (real terraform binary vs an in-process fake uapi; no router)
make fmt vet    # format and vet
make docs       # regenerate docs/ from schema descriptions + examples
```

CI runs build/test, golangci-lint, the docs-drift check, and the acceptance
suite on every push and PR.

## Conventions (see CLAUDE.md for the full contract)

- No `/raw`: only uapi's curated endpoints.
- No em-dashes anywhere; comments are rare and explain why, not what.
- New server-defaulted fields are `Optional + Computed`; only validate-required
  fields are `Required`. No client-side enum validators (uapi validates).
- Read responses into a map and pick known keys; never strict-decode (uapi adds
  response fields within a major).
- Every new attribute needs a `Description` (resource and data source), and the
  data-source schema must mirror the resource model field-for-field.
- Give new resources acceptance coverage (extend `mock_uapi_test.go` and the
  tables in `acceptance_test.go` / `coverage_test.go`). Regenerate and commit
  `docs/`.

## Verifying against real hardware

The acceptance suite uses a fake for determinism and CI. To exercise the
provider against a real router, build and use a dev override:

```sh
make install
export TF_CLI_CONFIG_FILE=$PWD/examples/dev.tfrc   # edit the path inside first
export UAPI_ENDPOINT=https://<router>/api/v1 UAPI_TOKEN=... UAPI_INSECURE=1
# then terraform plan/apply a config in a scratch dir
```

## Releases

Tag `vX.Y.Z` where `X.Y` matches the uapi `X.Y` the release covers. The release
workflow (GoReleaser) builds and GPG-signs the registry artifacts.
