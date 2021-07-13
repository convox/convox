variable "cluster" {
  type = string
}

variable "idle_timeout" {
  type = number
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
  type = list
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
