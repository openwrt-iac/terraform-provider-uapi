# Import a managed vnstat interface by its stable id.
terraform import uapi_vnstat_interface.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_vnstat_interface.example cfg0a1b2c
