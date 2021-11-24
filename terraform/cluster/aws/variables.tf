variable "arm_type" {
  default = false
}

variable "availability_zones" {
  default = ""
}

variable "cidr" {
  default = "10.1.0.0/16"
}

variable "gpu_type" {
  default = false
}

variable "high_availability" {
  default = true
}

variable "k8s_version" {
  type = string
  default = "1.18"
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
