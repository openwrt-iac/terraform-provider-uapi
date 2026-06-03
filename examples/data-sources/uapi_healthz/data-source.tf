data "uapi_healthz" "status" {}

output "uapi_status" {
  value = data.uapi_healthz.status.status
}
