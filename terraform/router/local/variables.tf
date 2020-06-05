variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "platform" {
  type = string
}

variable "release" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
