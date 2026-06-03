# Import a managed network interface by its stable id.
terraform import uapi_network_interface.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_network_interface.example cfg0a1b2c
