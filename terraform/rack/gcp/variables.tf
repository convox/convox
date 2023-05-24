variable "buildkit_enabled" {
  default = false
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

variable "image" {
  type = string
}

variable "name" {
  type = string
}

variable "rack_name" {
<<<<<<< HEAD
  default = ""
  type    = string
=======
  type = string
>>>>>>> 24b9e3c9 (Change rack name (#589))
}

variable "network" {
  type = string
}

variable "node_type" {
  default = "n1-standard-1"
}

variable "nodes_account" {
  type = string
}

variable "project_id" {
  type = string
}

variable "region" {
  default = "us-east1"
}

variable "release" {
  type = string
}

variable "settings" {
  type    = string
  default = ""
}

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type   = bool
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
