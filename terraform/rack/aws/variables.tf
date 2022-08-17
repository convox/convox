variable "cluster" {
  type = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "ebs_csi_driver_name" {
  type = string
}

variable "high_availability" {
  default = true
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

variable "proxy_protocol" {
  default = false
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
