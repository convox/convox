variable "annotations" {
  default = {}
  type = map(string)
}

variable "authentication" {
  default = true
  type = bool
}

variable "docker_hub_authentication" {
  default = null
  type    = string
}

variable "domain" {
  type = string
}

variable "env" {
  type = map(string)
  default = {}
}

variable "image" {
  type = string
}

variable "labels" {
  type = map(string)
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
  type = number
  default = 2
}

variable "set_priority_class" {
  type = bool
  default = true
}

variable "socket" {
  default = "/var/run/docker.sock"
  type = string
}

variable "volumes" {
  type = map(string)
  default = {}
}
