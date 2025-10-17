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
  default = ""
  type    = string
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

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type = bool
}

variable "telemetry_map" {
  type = any
}

variable "telemetry_default_map" {
  type = any
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "private_api" {
  type    = bool
  default = false
}
