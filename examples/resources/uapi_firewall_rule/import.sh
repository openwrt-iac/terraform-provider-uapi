# Import a managed firewall rule by its stable id.
terraform import uapi_firewall_rule.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_firewall_rule.example cfg0a1b2c
