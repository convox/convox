
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go exoscale

locals {
  telemetry_map = {
    build_node_enabled = var.build_node_enabled
    build_node_min_count = var.build_node_min_count
    build_node_type = var.build_node_type
    disable_image_manifest_cache = var.disable_image_manifest_cache
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    high_availability = var.high_availability
    image = var.image
    instance_disk_size = var.instance_disk_size
    instance_type = var.instance_type
    k8s_version = var.k8s_version
    name = var.name
    rack_name = var.rack_name
    registry_disk = var.registry_disk
    release = var.release
    ssl_ciphers = var.ssl_ciphers
    ssl_protocols = var.ssl_protocols
    telemetry = var.telemetry
    whitelist = var.whitelist
    zone = var.zone
    }

  telemetry_default_map = {
    build_node_enabled = "false"
    build_node_min_count = "0"
    build_node_type = ""
    disable_image_manifest_cache = "false"
    docker_hub_password = ""
    docker_hub_username = ""
    high_availability = "true"
    image = "convox/convox"
    instance_disk_size = "50"
    instance_type = "standard.medium"
    k8s_version = "1.28.4"
    name = ""
    rack_name = ""
    registry_disk = "50"
    release = ""
    ssl_ciphers = ""
    ssl_protocols = ""
    telemetry = "false"
    whitelist = "0.0.0.0/0"
    zone = "ch-gva-2"
    }
}
