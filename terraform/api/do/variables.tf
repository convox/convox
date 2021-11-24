variable "access_id" {
  type = string
}

variable "cluster" {
  type = string
}

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

variable "region" {
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

variable "secret_key" {
  type = string
}

variable "syslog" {
  default = ""
}
