variable "cluster" {
  type = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
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

variable "region" {
  type = string
}

variable "release" {
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

variable "settings" {
  type    = string
  default = ""
}

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type   = bool
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "workspace" {
  type = string
}
