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

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "release" {
  type = string
}

variable "replicas" {
  default = 2
}

variable "socket" {
  default = "/var/run/docker.sock"
}
