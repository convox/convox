variable "access_id" {
  type = string
}

variable "buildkit_enabled" {
  default = false
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

variable "high_availability" {
  default = true
}

variable "image" {
  default = "convox/convox"
}

variable "k8s_version" {
  type    = string
  default = "1.24"
}

variable "name" {
  type = string
}

variable "rack_name" {
  type = string
}

variable "node_type" {
  default = "s-2vcpu-4gb"
}

variable "region" {
  default = "nyc3"
}

variable "registry_disk" {
  default = "50Gi"
}

variable "release" {
  default = ""
}

variable "secret_key" {
  type = string
}

variable "settings" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type   = bool
  default = false
}

variable "token" {
  type = string
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
