ephemeral "uapi_token" "deploy" {
  name               = "ci-deploy"
  scopes             = ["network:write", "firewall:write"]
  expires_in_seconds = 3600
  allowed_cidrs      = ["10.0.0.0/24"]
}

# Use the minted token to configure a second provider instance, scoped to the run.
provider "uapi" {
  alias    = "scoped"
  endpoint = "https://router.example.com/api/v2"
  token    = ephemeral.uapi_token.deploy.token
}
