variable "annotations" {
  default = {}
  type    = map(string)
}

variable "cluster" {
  type = string
}

variable "env" {
  type    = map(string)
  default = {}
}

variable "image" {
  type = string
}

variable "rack" {
  type = string
}

variable "namespace" {
  type = string
}

variable "target" {
  type = string
}
