variable "docker_hub_authentication" {
  type = string
}

variable "domain" {
  type = string
}

variable "elasticsearch_disk" {
  type    = string
  default = "5Gi"
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
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

variable "release" {
  type = string
}

variable "resolver" {
  type = string
}

variable "router" {
  type = string
}

variable "secret" {
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

variable "private_api" {
  type    = bool
  default = false
}
