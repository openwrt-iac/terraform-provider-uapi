# Import a managed mwan3 member by its stable id.
terraform import uapi_mwan3_member.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_mwan3_member.example cfg0a1b2c
