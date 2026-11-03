data "uapi_wireguard_kernel_peers" "drift" {}

output "unclaimed_kernel_peers" {
  value = data.uapi_wireguard_kernel_peers.drift.peers
}
