
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go gcp

locals {
  telemetry_map = {
    additional_node_groups_config   = var.additional_node_groups_config
    buildkit_enabled                = var.buildkit_enabled
    cert_duration                   = var.cert_duration
    dcgm_scrape_interval            = var.dcgm_scrape_interval
    docker_hub_password             = var.docker_hub_password
    docker_hub_username             = var.docker_hub_username
    fluentd_memory                  = var.fluentd_memory
    gpu_observability_chart_version = var.gpu_observability_chart_version
    gpu_observability_enable        = var.gpu_observability_enable
    image                           = var.image
    k8s_version                     = var.k8s_version
    name                            = var.name
    nginx_additional_config         = var.nginx_additional_config
    node_disk                       = var.node_disk
    node_type                       = var.node_type
    preemptible                     = var.preemptible
    rack_name                       = var.rack_name
    region                          = var.region
    release                         = var.release
    settings                        = var.settings
    syslog                          = var.syslog
    telemetry                       = var.telemetry
    terraform_update_timeout        = var.terraform_update_timeout
    webhook_signing_key             = var.webhook_signing_key
    whitelist                       = var.whitelist
  }

  telemetry_default_map = {
    additional_node_groups_config   = ""
    buildkit_enabled                = "false"
    cert_duration                   = "2160h"
    dcgm_scrape_interval            = "15s"
    docker_hub_password             = ""
    docker_hub_username             = ""
    fluentd_memory                  = "200Mi"
    gpu_observability_chart_version = "4.8.1"
    gpu_observability_enable        = "false"
    image                           = "convox/convox"
    k8s_version                     = "1.35"
    name                            = ""
    nginx_additional_config         = ""
    node_disk                       = "100"
    node_type                       = "n1-standard-2"
    preemptible                     = "true"
    rack_name                       = ""
    region                          = "us-east1"
    release                         = ""
    settings                        = ""
    syslog                          = ""
    telemetry                       = "false"
    terraform_update_timeout        = "2h"
    webhook_signing_key             = ""
    whitelist                       = "0.0.0.0/0"
  }
}
