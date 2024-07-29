variable "docker_hub_authentication" {
  type    = string
  default = null
}

variable "env" {
  default = {}
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "network" {
  type = string
}

variable "release" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
