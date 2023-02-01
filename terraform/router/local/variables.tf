variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "platform" {
  type = string
}

variable "release" {
  type = string
}

variable "nginx_image" {
  type = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.3.0@sha256:d1707ca76d3b044ab8a28277a2466a02100ee9f58a86af1535a3edf9323ea1b5"
}
