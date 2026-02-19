variable "additional_node_groups" {
  type    = list(map(any))
  default = []
}

variable "additional_build_groups" {
  type    = list(map(any))
  default = []
}

variable "k8s_version" {
  type    = string
  default = "1.33"
}

variable "max_on_demand_count" {
  type    = number
  default = 100
}

variable "min_on_demand_count" {
  type    = number
  default = 3
}

variable "name" {
  type = string
}

variable "node_disk" {
  type    = number
  default = 30
}

variable "node_type" {
  type = string
}

variable "region" {
  type = string
}

variable "resource_group" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "nvidia_device_plugin_enable" {
  default = false
  type    = bool
}

variable "nvidia_device_time_slicing_replicas" {
  type    = number
  default = 0
}

variable "resource_group_location" {
  type = string
}
