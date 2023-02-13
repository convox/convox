variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "image" {
  default = "convox/convox"
}

variable "name" {
  type = string
}

variable "os" {
  default = "ubuntu"
}

variable "release" {
  default = ""
}
