variable "annotations" {
  type    = map(any)
  default = {}
}

variable "docker_hub_authentication" {
  type    = string
  default = null
}

variable "env" {
  type    = map(any)
  default = {}
}

variable "image" {
  type = string
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

