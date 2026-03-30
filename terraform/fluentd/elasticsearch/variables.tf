variable "cluster" {
  type = string
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "elasticsearch" {
  type = string
}

variable "namespace" {
  type = string
}

variable "rack" {
  type = string
}

variable "syslog" {
  default = ""
}
