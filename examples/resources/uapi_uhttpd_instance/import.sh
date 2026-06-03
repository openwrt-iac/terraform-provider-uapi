# Import a managed uhttpd instance by its stable id.
terraform import uapi_uhttpd_instance.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_uhttpd_instance.example cfg0a1b2c
