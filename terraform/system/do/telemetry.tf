
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go do

locals {
  telemetry_map = {
    buildkit_enabled         = var.buildkit_enabled
    cert_duration            = var.cert_duration
    docker_hub_password      = var.docker_hub_password
    docker_hub_username      = var.docker_hub_username
    elasticsearch_disk       = var.elasticsearch_disk
    fluentd_memory           = var.fluentd_memory
    high_availability        = var.high_availability
    image                    = var.image
    k8s_version              = var.k8s_version
    name                     = var.name
    node_type                = var.node_type
    private_api              = var.private_api
    rack_name                = var.rack_name
    region                   = var.region
    registry_disk            = var.registry_disk
    release                  = var.release
    router_log_redact_params = var.router_log_redact_params
    settings                 = var.settings
    syslog                   = var.syslog
    syslog_tls_verify        = var.syslog_tls_verify
    telemetry                = var.telemetry
    terraform_update_timeout = var.terraform_update_timeout
    webhook_signing_key      = var.webhook_signing_key
    whitelist                = var.whitelist
  }

  telemetry_default_map = {
    buildkit_enabled         = "false"
    cert_duration            = "2160h"
    docker_hub_password      = ""
    docker_hub_username      = ""
    elasticsearch_disk       = "5Gi"
    fluentd_memory           = "200Mi"
    high_availability        = "true"
    image                    = "convox/convox"
    k8s_version              = "1.35"
    name                     = ""
    node_type                = "s-2vcpu-4gb"
    private_api              = "false"
    rack_name                = ""
    region                   = "nyc3"
    registry_disk            = "50Gi"
    release                  = ""
    router_log_redact_params = ""
    settings                 = ""
    syslog                   = ""
    syslog_tls_verify        = "true"
    telemetry                = "false"
    terraform_update_timeout = "2h"
    webhook_signing_key      = ""
    whitelist                = "0.0.0.0/0"
  }
}
