variable "annotations" {
  default = {}
}

variable "authentication" {
  default = true
}

# skipcd
variable "buildkit_enabled" {
  default = false
  type    = bool
}

variable "docker_hub_authentication" {
  default = null
  type    = string
}

variable "domain" {
  type = string
}

variable "domain_internal" {
  type    = string
  default = ""
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
