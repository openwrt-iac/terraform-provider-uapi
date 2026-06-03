# Import a managed dropbear instance by its stable id.
terraform import uapi_dropbear_instance.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_dropbear_instance.example cfg0a1b2c
