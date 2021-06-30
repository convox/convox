variable "namespace" {
  type = string
}

variable "rack" {
  type = string
}

variable "replicas_min" {
  default = 2
}

variable "replicas_max" {
  default = 10
}

variable "set_priority_class" {
  default = true
}
