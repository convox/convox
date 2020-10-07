variable "cidr" {
  default = "10.1.0.0/16"
}

variable "kubernetes_version" {
  default = "1.13"
}

variable "max_node_group_size" {
  default = 100
}

variable "name" {
  type = string
}

variable "node_disk" {
  default = 20
}

variable "node_group_count" {
  default = 3
}

variable "node_type" {
  default = "t3.small"
}

variable "private" {
  default = true
}
