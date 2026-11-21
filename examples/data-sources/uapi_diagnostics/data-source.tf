data "uapi_diagnostics" "now" {}

output "uapi_uptime" {
  value = data.uapi_diagnostics.now.uptime_seconds
}

# A write that a killed request left after its commit is in uci unchecked.
check "no_interrupted_writes" {
  assert {
    condition     = try(length(data.uapi_diagnostics.now.interrupted_writes), 0) == 0
    error_message = "uapi reports an interrupted write: read data.uapi_diagnostics.now.interrupted_writes."
  }
}
