# Import a managed uhttpd cert by its stable id.
terraform import uapi_uhttpd_cert.example <id>

# Importing an anonymous (unmanaged) section adopts it (renames to a stable id).
terraform import uapi_uhttpd_cert.example cfg0a1b2c
