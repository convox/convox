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
