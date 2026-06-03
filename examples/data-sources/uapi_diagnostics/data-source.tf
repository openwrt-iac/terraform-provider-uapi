data "uapi_diagnostics" "now" {}

output "uapi_uptime" {
  value = data.uapi_diagnostics.now.uptime_seconds
}
