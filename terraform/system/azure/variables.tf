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
  default = "1.34"
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "max_on_demand_count" {
  type    = number
  default = 100
}

variable "min_on_demand_count" {
  type    = number
  default = 3
}

variable "node_disk" {
  type    = number
  default = 30
}

variable "node_type" {
  default = "Standard_D2_v3"
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
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

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "azure_files_enable" {
  default = false
  type    = bool
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
