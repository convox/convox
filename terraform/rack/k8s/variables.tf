variable "docker_hub_username" {
  type    = string
  default = ""
}

variable "docker_hub_password" {
  type    = string
  default = ""
}

variable "domain" {
  type = string
}

variable "name" {
  type = string
}

variable "release" {
  type = string
}

variable "telemetry" {
  type   = bool
}

variable "telemetry_file" {
  type    = string
  default = ""
}
