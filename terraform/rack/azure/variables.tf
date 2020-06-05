variable "cluster" {
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "workspace" {
  type = string
}
