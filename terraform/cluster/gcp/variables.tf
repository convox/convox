variable "additional_node_groups" {
  type    = list(map(any))
  default = []
}

variable "k8s_version" {
  type    = string
  default = "1.35"
}

variable "name" {
  type = string
}

variable "node_disk" {
  default = 100
}

variable "node_type" {
  type = string
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}

variable "preemptible" {
  default = true
}

variable "project_id" {
  type = string
}

variable "gpu_observability_enable" {
  type    = bool
  default = false
}

variable "gpu_observability_chart_version" {
  type    = string
  default = "4.8.1"
}

variable "dcgm_scrape_interval" {
  type    = string
  default = "15s"
}
