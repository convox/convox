variable "cluster" {
  type = string
}

variable "docker_hub_authentication" {
  type = string
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

variable "rack_name" {
  default = ""
  type    = string
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

variable "resource_group" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "resource_group_location" {
  type = string
}

variable "router" {
  type = string
}

variable "syslog" {
  default = ""
}

variable "workspace" {
  type = string
}

variable "private_api" {
  type    = bool
  default = false
}
