# Import a managed dhcp host by its stable id.
terraform import uapi_dhcp_host.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_dhcp_host.example cfg0a1b2c
