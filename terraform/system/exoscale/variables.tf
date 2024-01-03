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

variable "rack_name" {
  default = ""
  type    = string
}

variable "image" {
  default = "convox/convox"
}

variable "instance_type" {
  type = string
  default = "standard.medium"
}

variable "zone" {
  type = string
  default = "ch-gva-2"
}

variable "release" {
  default = ""
}

variable "instance_disk_size" {
  type = number
  default = 50
}
