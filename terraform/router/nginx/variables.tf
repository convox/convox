variable "docker_hub_authentication" {
  type    = string
  default = null
}

variable "internal_router" {
  type    = bool
  default = false
}

variable "nginx_image" {
  type    = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.12.0@sha256:e6b8de175acda6ca913891f0f727bca4527e797d52688cbe9fec9040d6f6b6fa"
}

variable "nginx_user" {
  type    = string
  default = "101"
}

variable "nginx_additional_config" {
  description = "Comma-separated key=value pairs (e.g., 'key1=value1,key2=value2')"
  type        = string
  default     = ""
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
