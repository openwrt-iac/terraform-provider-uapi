# Import a managed snmpd access by its stable id.
terraform import uapi_snmpd_access.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_snmpd_access.example cfg0a1b2c
