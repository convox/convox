variable "domain" {
  default = ""
}

variable "name" {
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
