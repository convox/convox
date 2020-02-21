variable "cluster" {
  type = string
}

variable "name" {
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

variable "subnets" {
  type = list
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
