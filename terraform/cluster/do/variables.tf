variable "high_availability" {
  default = true
}

variable "k8s_version" {
  type = string
  default = "1.22"
}

variable "name" {
  type = string
}

variable "node_type" {
  type = string
}

variable "region" {
  type = string
}
