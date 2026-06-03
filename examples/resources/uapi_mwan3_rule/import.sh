# Import a managed mwan3 rule by its stable id.
terraform import uapi_mwan3_rule.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_mwan3_rule.example cfg0a1b2c
