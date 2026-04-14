variable "cluster" {
  type = string
}

variable "azure_files_enabled" {
  default = false
  type    = bool
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "high_availability" {
  default = true
  type    = bool
}

variable "idle_timeout" {
  default = 4
  type    = number
}

variable "image" {
  type = string
}

variable "name" {
  type = string
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

variable "rack_name" {
  default = ""
  type    = string
}

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "resource_group" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "resource_group_location" {
  type = string
}

variable "syslog" {
  default = ""
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
  default = {}
  type    = map(string)
}

variable "telemetry" {
  type = bool
}

variable "telemetry_map" {
  type = any
}

variable "telemetry_default_map" {
  type = any
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "workspace" {
  type = string
}
