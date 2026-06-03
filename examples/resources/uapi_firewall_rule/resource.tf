resource "uapi_firewall_rule" "example" {
  target = "example"
  match = {
    src_zone = "example"
  }
}
