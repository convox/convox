variable "buildkit_enabled" {
  default = false
}

variable "cluster" {
  type = string
}

variable "cost_tracking_enable" {
  default = false
  type    = bool
}

variable "cost_tracking_history_days" {
  default = 62
  type    = number
}

variable "elasticsearch_disk" {
  type    = string
  default = "5Gi"
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "docker_hub_authentication" {
  type = string
}

variable "domain" {
  type = string
}

variable "image" {
  type = string
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "namespace" {
  type = string
}

variable "nodes_account" {
  type = string
}

variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "resolver" {
  type = string
}

variable "private_api" {
  type    = bool
  default = false
}

variable "router" {
  type = string
}

variable "router_log_redact_params" {
  type    = string
  default = ""
}

variable "syslog" {
  default = ""
}

variable "syslog_tls_verify" {
  type    = bool
  default = true
}

variable "webhook_signing_key" {
  type    = string
  default = ""
}
