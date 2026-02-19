variable "additional_build_groups_config" {
  type    = string
  default = ""
}

variable "additional_node_groups_config" {
  type    = string
  default = ""
}

variable "cert_duration" {
  default = "2160h"
  type    = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "image" {
  default = "convox/convox"
}

variable "k8s_version" {
  type    = string
  default = "1.33"
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "node_type" {
  default = "Standard_D2_v3"
}

variable "region" {
  default = "eastus"
}

variable "release" {
  default = ""
}

variable "settings" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type    = bool
  default = false
}

variable "nvidia_device_plugin_enable" {
  default = false
  type    = bool
}

variable "high_availability" {
  default = true
  type    = bool
}

variable "idle_timeout" {
  default = 4
  type    = number
}

variable "internal_router" {
  default = false
  type    = bool
}

variable "nvidia_device_time_slicing_replicas" {
  type    = number
  default = 0
}

variable "nginx_additional_config" {
  default = ""
  type    = string
}

variable "nginx_image" {
  default = ""
  type    = string
}

variable "pdb_default_min_available_percentage" {
  default = "50"
  type    = string
}

variable "proxy_protocol" {
  default = false
  type    = bool
}

variable "ssl_ciphers" {
  default = ""
  type    = string
}

variable "ssl_protocols" {
  default = ""
  type    = string
}

variable "tags" {
  default = ""
  type    = string
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
