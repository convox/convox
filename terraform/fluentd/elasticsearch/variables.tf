variable "cluster" {
  type = string
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "elasticsearch" {
  type = string
}

variable "namespace" {
  type = string
}

variable "rack" {
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
