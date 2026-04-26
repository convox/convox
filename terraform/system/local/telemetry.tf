
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go local

locals {
  telemetry_map = {
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    image               = var.image
    name                = var.name
    os                  = var.os
    rack_name           = var.rack_name
    release             = var.release
    settings            = var.settings
    telemetry           = var.telemetry
  }

  telemetry_default_map = {
    docker_hub_password = ""
    docker_hub_username = ""
    image               = "convox/convox"
    name                = ""
    os                  = "ubuntu"
    rack_name           = ""
    release             = ""
    settings            = ""
    telemetry           = "false"
  }
}
