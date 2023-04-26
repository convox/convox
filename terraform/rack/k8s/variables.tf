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

variable "settings" {
  type    = string
  default = ""
}

variable "telemetry" {
  type   = bool
}
