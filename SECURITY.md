# Security

## Reporting a vulnerability

Open a private security advisory on the GitHub repository
(`raspbeguy/terraform-provider-uapi`) or contact the maintainer directly. Please
do not file public issues for vulnerabilities.

## Secrets in Terraform state

Terraform state contains secrets. This provider specifically:

- Stores **write-preserved** secrets that uapi never returns but Terraform must
  keep to manage: `uapi_wireless_interface.key`, `uapi_network_interface.private_key`,
  `uapi_network_wireguard_peer.preshared_key`. These live in state (marked
  sensitive); the API exposes only a `has_*` flag.
- Does **not** store `uapi_system_password.password_wo`: it is a true write-only
  attribute, present only in config, never written to state.

Protect your state backend accordingly (encryption at rest, least-privilege
access). The provider `token` is likewise sensitive; prefer the `UAPI_TOKEN`
environment variable over committing it.

## Transport

uapi enforces TLS for non-localhost. `insecure = true` disables certificate
verification and is intended only for the default self-signed certificate during
bring-up; use a real certificate in production.
