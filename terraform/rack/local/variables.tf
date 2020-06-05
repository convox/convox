variable "name" {
  type = string
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
