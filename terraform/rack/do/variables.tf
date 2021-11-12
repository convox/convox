variable "access_id" {
  type = string
}

variable "cluster" {
  type = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "high_availability" {
  default = true
}

variable "image" {
  type = string
}

variable "name" {
  type = string
}

variable "region" {
  type = string
}

variable "registry_disk" {
  type = string
}

variable "release" {
  type = string
}

variable "secret_key" {
  type = string
}

variable "syslog" {
  default = ""
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
