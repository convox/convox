variable "annotations" {
  default = {}
}

variable "disable_api_k8s_proxy" {
  default = false
  type    = bool
}

variable "authentication" {
  default = true
}

# skipcd
variable "buildkit_enabled" {
  default = false
  type    = bool
}

variable "build_node_enabled" {
  default = false
  type    = bool
}

variable "convox_domain_tls_cert_disable" {
  default = false
  type    = bool
}

variable "docker_hub_authentication" {
  default = null
  type    = string
}

variable "docker_hub_username" {
  type    = string
  default = ""
}

variable "docker_hub_password" {
  type    = string
  default = ""
}

variable "domain" {
  type = string
}

variable "domain_internal" {
  type    = string
  default = ""
}

variable "disable_image_manifest_cache" {
  type    = bool
  default = false
}

variable "env" {
  default = {}
}

variable "image" {
  type = string
}

variable "labels" {
  default = {}
}

variable "metrics_scraper_host" {
  default = ""
  type    = string
}

variable "namespace" {
  type = string
}

variable "rack" {
  type = string
}

variable "release" {
  type = string
}

variable "resolver" {
  type = string
}

variable "replicas" {
  default = 2
}

variable "set_priority_class" {
  default = true
}

variable "socket" {
  default = "/var/run/docker.sock"
}

variable "volumes" {
  default = {}
}

variable "rack_name" {
  default = ""
  type    = string
}
