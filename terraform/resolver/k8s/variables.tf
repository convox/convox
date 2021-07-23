variable "annotations" {
  type    = map
  default = {}
}

variable "env" {
  type    = map
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

