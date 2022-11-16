variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "image" {
  default = "convox/convox"
}

variable "k8s_version" {
  type = string
  default = "1.23"
}

variable "name" {
  type = string
}

variable "node_type" {
  default = "n1-standard-2"
}

variable "preemptible" {
  default = true
}

variable "region" {
  default = "us-east1"
}

variable "release" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
