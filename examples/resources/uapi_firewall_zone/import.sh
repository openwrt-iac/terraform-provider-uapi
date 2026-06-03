# Import a managed firewall zone by its stable id.
terraform import uapi_firewall_zone.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_firewall_zone.example cfg0a1b2c
