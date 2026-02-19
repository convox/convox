variable "docker_hub_authentication" {
  type    = string
  default = null
}

variable "env" {
  default = {}
}

variable "high_availability" {
  default = true
  type    = bool
}

variable "idle_timeout" {
  default = 4
  type    = number
}

variable "internal_router" {
  default = false
  type    = bool
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "nginx_additional_config" {
  default = ""
  type    = string
}

variable "nginx_image" {
  default = ""
  type    = string
}

variable "proxy_protocol" {
  default = false
  type    = bool
}

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "ssl_ciphers" {
  default = ""
  type    = string
}

variable "ssl_protocols" {
  default = ""
  type    = string
}

variable "tags" {
  default = {}
  type    = map(string)
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
