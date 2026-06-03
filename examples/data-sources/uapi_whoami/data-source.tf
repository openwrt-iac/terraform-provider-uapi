data "uapi_whoami" "current" {}

output "token_scopes" {
  value = data.uapi_whoami.current.scopes
}
