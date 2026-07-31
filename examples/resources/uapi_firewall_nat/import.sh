# Import a managed firewall NAT rule by its stable id.
terraform import uapi_firewall_nat.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_firewall_nat.example cfg0a1b2c
