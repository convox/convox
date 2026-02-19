
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go azure

locals {
  telemetry_map = {
    cert_duration = var.cert_duration
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    high_availability = var.high_availability
    idle_timeout = var.idle_timeout
    image = var.image
    internal_router = var.internal_router
    k8s_version = var.k8s_version
    name = var.name
    nginx_additional_config = var.nginx_additional_config
    nginx_image = var.nginx_image
    node_type = var.node_type
    nvidia_device_plugin_enable = var.nvidia_device_plugin_enable
    nvidia_device_time_slicing_replicas = var.nvidia_device_time_slicing_replicas
    pdb_default_min_available_percentage = var.pdb_default_min_available_percentage
    proxy_protocol = var.proxy_protocol
    rack_name = var.rack_name
    region = var.region
    release = var.release
    settings = var.settings
    ssl_ciphers = var.ssl_ciphers
    ssl_protocols = var.ssl_protocols
    syslog = var.syslog
    tags = var.tags
    telemetry = var.telemetry
    whitelist = var.whitelist
    }

  telemetry_default_map = {
    cert_duration = "2160h"
    docker_hub_password = ""
    docker_hub_username = ""
    high_availability = "true"
    idle_timeout = "4"
    image = "convox/convox"
    internal_router = "false"
    k8s_version = "1.33"
    name = ""
    nginx_additional_config = ""
    nginx_image = ""
    node_type = "Standard_D2_v3"
    nvidia_device_plugin_enable = "false"
    nvidia_device_time_slicing_replicas = "0"
    pdb_default_min_available_percentage = "50"
    proxy_protocol = "false"
    rack_name = ""
    region = "eastus"
    release = ""
    settings = ""
    ssl_ciphers = ""
    ssl_protocols = ""
    syslog = ""
    tags = ""
    telemetry = "false"
    whitelist = "0.0.0.0/0"
    }
}
