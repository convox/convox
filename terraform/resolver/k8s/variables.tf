variable "annotations" {
  type    = map(any)
  default = {}
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
