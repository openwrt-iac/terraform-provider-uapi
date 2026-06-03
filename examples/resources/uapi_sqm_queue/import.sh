# Import a managed sqm queue by its stable id.
terraform import uapi_sqm_queue.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_sqm_queue.example cfg0a1b2c
