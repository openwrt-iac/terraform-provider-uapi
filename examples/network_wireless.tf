# A bridge device with a static interface on top, and a wireless AP.

resource "uapi_network_device" "br_lan" {
  name  = "br-lan"
  type  = "bridge"
  ports = ["eth0", "eth1"]
}

resource "uapi_network_interface" "lan" {
  device  = uapi_network_device.br_lan.name
  proto   = "static"
  ipaddr  = "192.168.1.1"
  netmask = "255.255.255.0"
  dns     = ["1.1.1.1", "9.9.9.9"]
}

resource "uapi_wireless_device" "radio0" {
  type    = "mac80211"
  band    = "5g"
  channel = "36"
  htmode  = "VHT80"
  country = "FR"
}

resource "uapi_wireless_interface" "home" {
  device     = uapi_wireless_device.radio0.id
  network    = uapi_network_interface.lan.id
  mode       = "ap"
  ssid       = "home-net"
  encryption = "psk2"
  key        = var.wifi_key
}

variable "wifi_key" {
  type      = string
  sensitive = true
}
