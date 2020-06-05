variable "cidr" {
  default = "10.1.0.0/16"
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

variable "release" {
  default = ""
}

variable "region" {
  default = "us-east-1"
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
