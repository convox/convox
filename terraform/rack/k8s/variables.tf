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

// for eks addons dependency
variable "eks_addons" { # skipcq
  default = []
}

variable "name" {
  type = string
}

variable "release" {
  type = string
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
