variable "env" {
  default = {}
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "oidc_arn" {
  type = string
}

variable "oidc_sub" {
  type = string
}

variable "release" {
  type = string
}

variable "resolver_target" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
