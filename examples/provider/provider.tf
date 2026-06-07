terraform {
  required_providers {
    uapi = {
      source = "openwrt-iac/uapi"
    }
  }
}

provider "uapi" {
  endpoint = "https://192.168.1.1/api/v1" # or env UAPI_ENDPOINT / UAPI_BASE
  token    = var.uapi_token               # or env UAPI_TOKEN

  # uapi ships a self-signed certificate by default. Set this for a quick start;
  # use a real certificate (acme.sh / luci-app-acme) in production.
  insecure = true # or env UAPI_INSECURE=1
}

variable "uapi_token" {
  type      = string
  sensitive = true
}
