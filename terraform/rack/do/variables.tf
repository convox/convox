variable "access_id" {
  type = string
}

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

variable "high_availability" {
  default = true
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
