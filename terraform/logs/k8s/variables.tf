variable "annotations" {
  default = {}
}

variable "env" {
  default = {}
}

variable "cluster" {
  type = "string"
}

variable "image" {
  type = "string"
}

variable "namespace" {
  type = "string"
}

variable "target" {
  type = "string"
}
