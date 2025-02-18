
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go do

locals {
  telemetry_map = {
    access_id = var.access_id
    buildkit_enabled = var.buildkit_enabled
    cert_duration = var.cert_duration
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    high_availability = var.high_availability
    image = var.image
    k8s_version = var.k8s_version
    name = var.name
    node_type = var.node_type
    rack_name = var.rack_name
    region = var.region
    registry_disk = var.registry_disk
    release = var.release
    secret_key = var.secret_key
    settings = var.settings
    syslog = var.syslog
    telemetry = var.telemetry
    token = var.token
    whitelist = var.whitelist
    }

  telemetry_default_map = {
    access_id = ""
    buildkit_enabled = "false"
    cert_duration = "2160h"
    docker_hub_password = ""
    docker_hub_username = ""
    high_availability = "true"
    image = "convox/convox"
    k8s_version = "1.30"
    name = ""
    node_type = "s-2vcpu-4gb"
    rack_name = ""
    region = "nyc3"
    registry_disk = "50Gi"
    release = ""
    secret_key = ""
    settings = ""
    syslog = ""
    telemetry = "false"
    token = ""
    whitelist = "0.0.0.0/0"
    }
}
