# Import a managed system timeserver by its stable id.
terraform import uapi_system_timeserver.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_system_timeserver.example cfg0a1b2c
