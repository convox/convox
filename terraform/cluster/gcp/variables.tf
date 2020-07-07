variable "name" {
  type = string
}

variable "node_type" {
  type = string
}

variable "preemptible" {
  default = true
}

variable "services" {
  default = ""
}
