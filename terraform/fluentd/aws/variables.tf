variable "access_log_retention_in_days" {
  default = "7"
  type    = string
}

variable "cluster" {
  type = string
}

variable "app_cloudwatch_disable" {
  type    = bool
  default = false
}

variable "fluentd_disable" {
  type    = bool
  default = false
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

// for eks addons dependency
variable "eks_addons" {}

variable "namespace" {
  type = string
}

variable "rack" {
  type = string
}

variable "oidc_arn" {
  type = string
}

variable "oidc_sub" {
  type = string
}

variable "syslog" {
  default = ""
}
