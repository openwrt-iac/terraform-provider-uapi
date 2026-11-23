# Start dnsmasq again after a write warned that procd gave up on it:
#   terraform apply -invoke=action.uapi_service_recover.dnsmasq
action "uapi_service_recover" "dnsmasq" {
  config {
    service = "dnsmasq"
  }
}

# Or run it after each update of the DHCP settings.
resource "uapi_dhcp_dnsmasq" "this" {
  lifecycle {
    action_trigger {
      events  = [after_update]
      actions = [action.uapi_service_recover.dnsmasq]
    }
  }
}

# Each instance that procd gave up on, until a recover or a restart.
data "uapi_diagnostics" "this" {}

check "no_stopped_daemon" {
  assert {
    condition     = length(data.uapi_diagnostics.this.stopped_instances) == 0
    error_message = "procd gave up on: ${join(", ", [for i in data.uapi_diagnostics.this.stopped_instances : "${i.service}/${i.instance}"])}"
  }
}
