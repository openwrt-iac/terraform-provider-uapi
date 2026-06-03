# Import a managed network bridge VLAN by its stable id.
terraform import uapi_network_bridge_vlan.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_network_bridge_vlan.example cfg0a1b2c
