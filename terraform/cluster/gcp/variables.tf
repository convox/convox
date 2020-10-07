variable "max_node_pool_size" {
  default = 1000
}

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
