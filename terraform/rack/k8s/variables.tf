variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
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
  default = ""
}

variable "telemetry_file" {
  type    = string
  default = ""
}
