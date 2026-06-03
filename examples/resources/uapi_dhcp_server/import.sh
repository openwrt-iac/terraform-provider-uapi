# Import a managed dhcp server by its stable id.
terraform import uapi_dhcp_server.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_dhcp_server.example cfg0a1b2c
