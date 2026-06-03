# Import a managed network rule by its stable id.
terraform import uapi_network_rule.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_network_rule.example cfg0a1b2c
