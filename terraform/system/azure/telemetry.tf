
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go azure

locals {
  telemetry_map = {
    cert_duration = var.cert_duration
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    image = var.image
    k8s_version = var.k8s_version
    name = var.name
    node_type = var.node_type
    rack_name = var.rack_name
    region = var.region
    release = var.release
    settings = var.settings
    syslog = var.syslog
    telemetry = var.telemetry
    whitelist = var.whitelist
    }

  telemetry_default_map = {
    cert_duration = "2160h"
    docker_hub_password = ""
    docker_hub_username = ""
    image = "convox/convox"
    k8s_version = "1.31"
    name = ""
    node_type = "Standard_D2_v3"
    rack_name = ""
    region = "eastus"
    release = ""
    settings = ""
    syslog = ""
    telemetry = "false"
    whitelist = "0.0.0.0/0"
    }
}
