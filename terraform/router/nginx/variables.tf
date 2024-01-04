variable "cluster_id" {
  type = string
  default = ""
}

variable "internal_router" {
  type    = bool
  default = false
}

variable "nginx_image" {
  type    = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.3.0@sha256:d1707ca76d3b044ab8a28277a2466a02100ee9f58a86af1535a3edf9323ea1b5"
}

variable "nginx_user" {
  type    = string
  default = "101"
}

variable "namespace" {
  type = string
}

variable "proxy_protocol" {
  default = false
}

variable "cloud_provider" {
  default = ""
}

variable "rack" {
  type = string
}

variable "replicas_min" {
  default = 2
}

variable "replicas_max" {
  default = 10
}

variable "set_priority_class" {
  default = true
}

variable "ssl_ciphers" {
  default = ""
  type    = string
}

variable "ssl_protocols" {
  default = ""
  type    = string
}
