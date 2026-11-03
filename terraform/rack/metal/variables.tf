variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "domain" {
  default = ""
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

variable "registry_disk" {
  default = "50Gi"
}

variable "release" {
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "private_api" {
  type    = bool
  default = false
}
