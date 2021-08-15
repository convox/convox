variable "image" {
  default = "convox/convox"
}

variable "kubeconfig" {
  default = "~/.kube/config"
}

variable "name" {
  type = string
}

variable "release" {
  default = ""
}
