variable "name" {
  type = string
}

variable "node_type" {
  type = string
}

variable "preemptible" {
  default = true
}

variable "services" {
  default = ""
}

variable "cluster_ca_certificate" {
  type = string
}
variable "host" {
  type = string
}
variable "token" {
  type = string
}
variable "kubeconfig_raw" {
  type = string
}
