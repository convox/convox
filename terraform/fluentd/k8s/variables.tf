variable "annotations" {
  default = {}
}

variable "cluster" {
  type = string
}

variable "env" {
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
