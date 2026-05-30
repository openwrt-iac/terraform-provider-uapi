# A firewall zone, plus a rule and a port forward that reference it.

resource "uapi_firewall_zone" "dmz" {
  name    = "dmz"
  input   = "DROP"
  output  = "ACCEPT"
  forward = "DROP"
  network = ["dmz"]
  masq    = true
}

resource "uapi_firewall_rule" "allow_ssh_from_wan" {
  name    = "Allow-SSH-from-WAN"
  target  = "ACCEPT"
  enabled = true

  match = {
    src_zone  = "wan"
    proto     = ["tcp"]
    dest_port = ["22"]
  }
}

resource "uapi_firewall_redirect" "web" {
  name   = "Forward-HTTP"
  target = "DNAT"

  match = {
    src_zone  = "wan"
    proto     = ["tcp"]
    src_dport = ["80"]
    dest_ip   = ["192.168.1.10"]
    dest_port = ["8080"]
  }
}
