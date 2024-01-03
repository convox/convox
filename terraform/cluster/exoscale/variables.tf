variable "high_availability" {
  default = true
}

variable "k8s_version" {
  type = string
  default = "1.28.4"
}

variable "name" {
  type = string
}

variable "instance_type" {
  type = string
}

variable "zone" {
  type = string
}

variable "instance_disk_size" {
  type = number
  default = 50
}
