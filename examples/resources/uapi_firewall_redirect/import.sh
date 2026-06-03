# Import a managed firewall redirect by its stable id.
terraform import uapi_firewall_redirect.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_firewall_redirect.example cfg0a1b2c
