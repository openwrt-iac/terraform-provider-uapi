# Import a managed snmpd com2sec by its stable id.
terraform import uapi_snmpd_com2sec.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_snmpd_com2sec.example cfg0a1b2c
