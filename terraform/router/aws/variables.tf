variable "env" {
  default = {}
}

variable "idle_timeout" {
  type = number
}

variable "high_availability" {
  default = true
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

variable "proxy_protocol" {
  default = false
}

variable "release" {
  type = string
}

variable "tags" {
  default = {}
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
