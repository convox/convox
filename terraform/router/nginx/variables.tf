variable "nginx_image" {
  type = string
  default = "k8s.gcr.io/ingress-nginx/controller:v0.49.3@sha256:35fe394c82164efa8f47f3ed0be981b3f23da77175bbb8268a9ae438851c8324"
}

variable "nginx_user" {
  type = string
  default = "101"
}

variable "namespace" {
  type = string
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
