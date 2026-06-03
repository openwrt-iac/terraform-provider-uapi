# Import a managed network route by its stable id.
terraform import uapi_network_route.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_network_route.example cfg0a1b2c
