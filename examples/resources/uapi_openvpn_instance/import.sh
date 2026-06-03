# Import a managed openvpn instance by its stable id.
terraform import uapi_openvpn_instance.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_openvpn_instance.example cfg0a1b2c
