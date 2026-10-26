variable "karpenter_enabled" {
  type    = bool
  default = false
}

variable "seccomp_default_enabled" {
  type    = bool
  default = false
}

variable "set_priority_class" {
  default = true
}
