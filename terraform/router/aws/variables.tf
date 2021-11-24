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

variable "release" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
