
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go gcp

locals {
  telemetry_map = {
    buildkit_enabled = var.buildkit_enabled
    cert_duration = var.cert_duration
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    image = var.image
    k8s_version = var.k8s_version
    name = var.name
    node_type = var.node_type
    preemptible = var.preemptible
    rack_name = var.rack_name
    region = var.region
    release = var.release
    settings = var.settings
    syslog = var.syslog
    telemetry = var.telemetry
    whitelist = var.whitelist
    }

  telemetry_default_map = {
    buildkit_enabled = "false"
    cert_duration = "2160h"
    docker_hub_password = ""
    docker_hub_username = ""
    image = "convox/convox"
    k8s_version = "1.30"
    name = ""
    node_type = "n1-standard-2"
    preemptible = "true"
    rack_name = ""
    region = "us-east1"
    release = ""
    settings = ""
    syslog = ""
    telemetry = "false"
    whitelist = "0.0.0.0/0"
    }
}
