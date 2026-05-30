# A static DHCP lease, global system settings, and reading runtime leases.

resource "uapi_dhcp_host" "printer" {
  name = "printer"
  mac  = "aa:bb:cc:dd:ee:ff"
  ip   = "192.168.1.50"
  dns  = true
}

resource "uapi_system" "this" {
  hostname = "edge-router"
  timezone = "CET-1CEST,M3.5.0,M10.5.0/3"
  zonename = "Europe/Paris"
}

# Read-only: the current active DHCP leases.
data "uapi_dhcp_leases" "current" {}

output "active_leases" {
  value = data.uapi_dhcp_leases.current.leases
}
