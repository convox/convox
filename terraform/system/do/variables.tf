variable "access_id" {
  type = string
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

variable "token" {
  type = string
}
