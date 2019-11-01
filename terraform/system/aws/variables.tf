variable "cidr" {
  default = "10.1.0.0/16"
}

variable "name" {
  type = "string"
}

variable "node_type" {
  default = "t3.small"
}

variable "release" {
  default = ""
}

variable "region" {
  default = "us-east-1"
}

variable "ssh_key" {
  default = ""
}
