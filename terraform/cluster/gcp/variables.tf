variable "k8s_version" {
  type    = string
  default = "1.22"
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

variable "preemptible" {
  default = true
}

variable "project_id" {
  type = string
}
