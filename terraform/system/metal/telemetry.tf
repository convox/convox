
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go metal

locals {
  telemetry_map = {
    docker_hub_password      = var.docker_hub_password
    docker_hub_username      = var.docker_hub_username
    domain                   = var.domain
    elasticsearch_disk       = var.elasticsearch_disk
    fluentd_memory           = var.fluentd_memory
    image                    = var.image
    name                     = var.name
    private_api              = var.private_api
    rack_name                = var.rack_name
    registry_disk            = var.registry_disk
    release                  = var.release
    router_log_redact_params = var.router_log_redact_params
    syslog                   = var.syslog
    syslog_tls_verify        = var.syslog_tls_verify
    webhook_signing_key      = var.webhook_signing_key
    whitelist                = var.whitelist
  }

  telemetry_default_map = {
    docker_hub_password      = ""
    docker_hub_username      = ""
    domain                   = ""
    elasticsearch_disk       = "5Gi"
    fluentd_memory           = "200Mi"
    image                    = "convox/convox"
    name                     = ""
    private_api              = "false"
    rack_name                = ""
    registry_disk            = "50Gi"
    release                  = ""
    router_log_redact_params = ""
    syslog                   = ""
    syslog_tls_verify        = "true"
    webhook_signing_key      = ""
    whitelist                = "0.0.0.0/0"
  }
}
