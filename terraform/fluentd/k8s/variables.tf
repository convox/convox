variable "annotations" {
  default = {}
}

variable "cluster" {
  type = string
}

variable "env" {
  default = {}
}

variable "fluentd_disable" {
  type    = bool
  default = false
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

variable "syslog" {
  default = ""
}

variable "target" {
  type = string
}
