variable "high_availability" {
  default = true
}

variable "k8s_version" {
  type    = string
  default = "1.34"
}

variable "name" {
  type = string
}

variable "node_type" {
  type = string
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}

variable "region" {
  type = string
}
