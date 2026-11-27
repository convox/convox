variable "docker_hub_authentication" {
  type    = string
  default = null
}

variable "high_availability" {
  default = true
}

variable "lb_bandwidth_max" {
  default = "100"
  type    = string
}

variable "lb_bandwidth_min" {
  default = "10"
  type    = string
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
