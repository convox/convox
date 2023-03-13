variable "buildkit_enabled" {
  default = false
}

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

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "resource_group" {
  type = string
}

variable "syslog" {
  default = ""
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "workspace" {
  type = string
}
