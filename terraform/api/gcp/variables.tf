variable "buildkit_enabled" {
  default = false
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

variable "image" {
  type = string
}

variable "name" {
  type = string
}

variable "rack_name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "nodes_account" {
  type = string
}

variable "project_id" {
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

variable "syslog" {
  default = ""
}
