# Import a managed snmpd group by its stable id.
terraform import uapi_snmpd_group.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_snmpd_group.example cfg0a1b2c
