variable "annotations" {
  default = {}
}

variable "authentication" {
  default = true
}

variable "docker_hub_authentication" {
  default = null
  type = string
}

variable "domain" {
  type = string
}

variable "env" {
  default = {}
}

variable "image" {
  type = string
}

variable "labels" {
  default = {}
}

variable "namespace" {
  type = string
}

variable "rack" {
  type = string
}

variable "release" {
  type = string
}

variable "resolver" {
  type = string
}

variable "replicas" {
  default = 2
}

variable "set_priority_class" {
  default = true
}

variable "socket" {
  default = "/var/run/docker.sock"
}

variable "volumes" {
  default = {}
}
