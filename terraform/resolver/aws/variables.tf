variable "docker_hub_authentication" {
  type = string
}

variable "high_availability" {
  default = true
}

variable "image" {
  type = string
}

variable "karpenter_enabled" {
  type    = bool
  default = false
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
