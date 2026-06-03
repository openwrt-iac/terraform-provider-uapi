# Import a managed snmpd agent by its stable id.
terraform import uapi_snmpd_agent.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_snmpd_agent.example cfg0a1b2c
