
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go metal

locals {
  telemetry_map = {
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    domain              = var.domain
    fluentd_memory      = var.fluentd_memory
    image               = var.image
    name                = var.name
    rack_name           = var.rack_name
    registry_disk       = var.registry_disk
    release             = var.release
    syslog              = var.syslog
    webhook_signing_key = var.webhook_signing_key
    whitelist           = var.whitelist
  }

  telemetry_default_map = {
    docker_hub_password = ""
    docker_hub_username = ""
    domain              = ""
    fluentd_memory      = "200Mi"
    image               = "convox/convox"
    name                = ""
    rack_name           = ""
    registry_disk       = "50Gi"
    release             = ""
    syslog              = ""
    webhook_signing_key = ""
    whitelist           = "0.0.0.0/0"
  }
}
