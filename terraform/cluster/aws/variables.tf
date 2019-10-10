variable "cidr" {
  default = "10.1.0.0/16"
}

variable "kubernetes_version" {
  default = "1.13"
}

variable "name" {
  type = "string"
}

variable "node_type" {
  default = "t3.small"
}

variable "ssh_key" {
  default = ""
}
