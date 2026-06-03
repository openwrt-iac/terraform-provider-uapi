# Import a managed firewall forwarding by its stable id.
terraform import uapi_firewall_forwarding.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_firewall_forwarding.example cfg0a1b2c
