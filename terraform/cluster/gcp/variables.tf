variable "k8s_version" {
  type    = string
  default = "1.22"
}

variable "name" {
  type = string
}

variable "node_type" {
  type = string
}

variable "preemptible" {
  type    = bool
  default = true
}

variable "project_id" {
  type = string
}
