variable "cert_manager" {
  type = bool
}

variable "domain" {
  type = string
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

variable "release" {
  type = string
}

variable "resolver" {
  type = string
}

variable "router" {
  type = string
}

variable "secret" {
  type = string
}

variable "syslog" {
  default = ""
}
