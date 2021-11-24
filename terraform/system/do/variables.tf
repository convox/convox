variable "access_id" {
  type = string
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

variable "name" {
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

variable "syslog" {
  default = ""
}

variable "token" {
  type = string
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
