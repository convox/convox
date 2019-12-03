variable "annotations" {
  type    = map
  default = {}
}

variable "env" {
  type    = map
  default = {}
}

variable "namespace" {
  type = string
}

variable "release" {
  type = string
}
