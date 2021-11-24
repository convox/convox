variable "arm_type" {
  default = false
}

variable "cluster" {
  type = string
}

variable "namespace" {
  type = string
}

variable "rack" {
  type = string
}

variable "oidc_arn" {
  type = string
}

variable "oidc_sub" {
  type = string
}

variable "syslog" {
  default = ""
}
