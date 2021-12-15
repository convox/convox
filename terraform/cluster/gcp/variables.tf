variable "name" {
  type = string
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

variable "services" {
  default = ""
}
