# Import a managed mwan3 policy by its stable id.
terraform import uapi_mwan3_policy.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_mwan3_policy.example cfg0a1b2c
