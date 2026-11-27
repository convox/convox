variable "compartment_ocid" {
  type = string
}

variable "gpu_node_count" {
  default = 1
}

# empty disables the gpu node pool and the nvidia device plugin
variable "gpu_node_type" {
  default = ""
  type    = string
}

variable "k8s_version" {
  type    = string
  default = "1.35"
}

variable "name" {
  type = string
}

variable "node_count" {
  default = 2
}

variable "node_disk" {
  default = 100
}

variable "node_memory" {
  default = 16
}

variable "node_ocpus" {
  default = 2
}

variable "node_type" {
  type = string
}

variable "region" {
  type = string
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}
