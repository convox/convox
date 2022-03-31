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
  type = string
  default = "1.20"
}

variable "name" {
  type = string
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

variable "syslog" {
  default = ""
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
