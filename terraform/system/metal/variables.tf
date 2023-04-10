variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "domain" {
  default = ""
}

variable "image" {
  default = "convox/convox"
}

variable "name" {
  type = string
}

variable "rack_name" {
  type = string
}

variable "registry_disk" {
  default = "50Gi"
}

variable "release" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
