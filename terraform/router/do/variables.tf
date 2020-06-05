variable "env" {
  default = {}
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
