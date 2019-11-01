variable "credentials" {
  default = "~/.config/gcloud/terraform.json"
}

variable "kubeconfig" {
  type = "string"
}

variable "name" {
  type = "string"
}

variable "node_type" {
  default = "n1-standard-1"
}

variable "nodes_account" {
  type = "string"
}

variable "region" {
  default = "us-east1"
}

variable "release" {
  type = "string"
}
