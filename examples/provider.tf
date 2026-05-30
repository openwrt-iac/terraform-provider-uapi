terraform {
  required_providers {
    uapi = {
      source = "raspbeguy/uapi"
    }
  }
}

provider "uapi" {
  endpoint = "https://192.168.1.1/api/v1"
  token    = var.uapi_token

  # uapi ships a self-signed certificate by default. Set this to true for a
  # quick start; use a real certificate (acme.sh / luci-app-acme) in production.
  insecure = true
}

variable "uapi_token" {
  type      = string
  sensitive = true
}
