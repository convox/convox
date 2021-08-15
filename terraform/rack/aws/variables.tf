variable "cluster" {
  type = string
}

variable "idle_timeout" {
  type = number
}

variable "image" {
  type = string
}

variable "name" {
  type = string
}

variable "oidc_arn" {
  type = string
}

variable "oidc_sub" {
  type = string
}

variable "release" {
  type = string
}

variable "subnets" {
  type = list(any)
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
