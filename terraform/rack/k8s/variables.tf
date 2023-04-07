variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
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
