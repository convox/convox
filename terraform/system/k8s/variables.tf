variable "kubeconfig" {
  type = "string"
}

variable "namespace" {
  type = "string"
}

variable "router_annotations" {
  type    = "map"
  default = {}
}

variable "router_env" {
  type    = "map"
  default = {}
}
