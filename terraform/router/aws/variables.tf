variable "convox_rack_domain" {
  default = ""
  type    = string
}

variable "deploy_extra_nlb" {
  default = false
  type    = bool
}

variable "docker_hub_authentication" {
  type = string
  default = null
}

variable "env" {
  default = {}
}

variable "idle_timeout" {
  type = number
}

variable "internal_router" {
  type    = bool
  default = false
}

variable "high_availability" {
  default = true
}

variable "lbc_helm_id" {
  default = ""
  type    = string
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "nlb_security_group" {
  default = ""
  type    = string
}

variable "nginx_image" {
  type    = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.3.0@sha256:d1707ca76d3b044ab8a28277a2466a02100ee9f58a86af1535a3edf9323ea1b5"
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
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
