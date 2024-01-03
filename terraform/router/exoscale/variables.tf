variable "high_availability" {
  default = true
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "proxy_protocol" {
  default = false
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
