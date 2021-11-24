variable "docker_hub_authentication" {
  type = string
}

variable "domain" {
  type = string
}

variable "high_availability" {
  default = true
}

variable "image" {
  type = string
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

variable "resolver" {
  type = string
}

variable "router" {
  type = string
}
