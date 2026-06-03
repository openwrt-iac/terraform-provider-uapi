# Import a managed network device by its stable id.
terraform import uapi_network_device.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_network_device.example cfg0a1b2c
