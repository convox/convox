
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go azure

locals {
  telemetry_map = {
    additional_build_groups_config = var.additional_build_groups_config
    additional_node_groups_config = var.additional_node_groups_config
    azure_files_enable = var.azure_files_enable
    cert_duration = var.cert_duration
    docker_hub_password = var.docker_hub_password
    docker_hub_username = var.docker_hub_username
    high_availability = var.high_availability
    idle_timeout = var.idle_timeout
    image = var.image
    k8s_version = var.k8s_version
    max_on_demand_count = var.max_on_demand_count
    min_on_demand_count = var.min_on_demand_count
    name = var.name
    nginx_additional_config = var.nginx_additional_config
    nginx_image = var.nginx_image
    node_disk = var.node_disk
    node_type = var.node_type
    nvidia_device_plugin_enable = var.nvidia_device_plugin_enable
    nvidia_device_time_slicing_replicas = var.nvidia_device_time_slicing_replicas
    pdb_default_min_available_percentage = var.pdb_default_min_available_percentage
    rack_name = var.rack_name
    region = var.region
    release = var.release
    settings = var.settings
    ssl_ciphers = var.ssl_ciphers
    ssl_protocols = var.ssl_protocols
    syslog = var.syslog
    tags = var.tags
    telemetry = var.telemetry
    terraform_update_timeout = var.terraform_update_timeout
    whitelist = var.whitelist
    }

  telemetry_default_map = {
    additional_build_groups_config = ""
    additional_node_groups_config = ""
    azure_files_enable = "false"
    cert_duration = "2160h"
    docker_hub_password = ""
    docker_hub_username = ""
    high_availability = "true"
    idle_timeout = "4"
    image = "convox/convox"
    k8s_version = "1.34"
    max_on_demand_count = "100"
    min_on_demand_count = "3"
    name = ""
    nginx_additional_config = ""
    nginx_image = ""
    node_disk = "30"
    node_type = "Standard_D2_v3"
    nvidia_device_plugin_enable = "false"
    nvidia_device_time_slicing_replicas = "0"
    pdb_default_min_available_percentage = "50"
    rack_name = ""
    region = "eastus"
    release = ""
    settings = ""
    ssl_ciphers = ""
    ssl_protocols = ""
    syslog = ""
    tags = ""
    telemetry = "false"
    terraform_update_timeout = "2h"
    whitelist = "0.0.0.0/0"
    }
}
