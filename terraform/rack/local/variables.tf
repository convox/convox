
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

variable "os" {
  default = "ubuntu"
}

variable "platform" {
  type = string
}

variable "registry_disk" {
  default = 20
}

variable "release" {
  type = string
}

variable "settings" {
  type    = string
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