resource "uapi_firewall_redirect" "example" {
  match = {
    src_zone = "example"
  }
}
