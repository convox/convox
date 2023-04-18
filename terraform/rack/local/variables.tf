
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

variable "telemetry" {
  type   = bool
}

variable "telemetry_file" {
  type    = string
  default = ""
}
