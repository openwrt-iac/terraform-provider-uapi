# Import a managed network WireGuard peer by its stable id.
terraform import uapi_network_wireguard_peer.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_network_wireguard_peer.example cfg0a1b2c
