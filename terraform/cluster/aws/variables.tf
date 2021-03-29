variable "cidr" {
  default = "10.1.0.0/16"
}

variable "exclude_zones" {
  default = ""
}

variable "kubernetes_version" {
  default = "1.13"
}

variable "name" {
  type = string
}

variable "node_disk" {
  default = 20
}

variable "node_type" {
  default = "t3.small"
}

variable "private" {
  default = true
}
