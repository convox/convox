variable "annotations" {
  default = {}
}

variable "authentication" {
  default = true
}

variable "domain" {
  type = string
}

variable "env" {
  default = {}
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
