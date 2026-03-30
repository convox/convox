provider "azurerm" {
  features {}
}

provider "kubernetes" {
  client_certificate     = module.cluster.client_certificate
  client_key             = module.cluster.client_key
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

}

data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  name      = lower(var.name)
  rack_name = lower(var.rack_name)
  current   = jsondecode(data.http.releases.response_body).tag_name
  release   = coalesce(var.release, local.current)

  tags_string = var.tags != "" ? try(base64decode(var.tags), var.tags) : ""
  tags = local.tags_string != "" ? {
    for item in split(",", local.tags_string) :
    split("=", item)[0] => split("=", item)[1]
  } : {}
  additional_node_groups  = try(jsondecode(var.additional_node_groups_config), jsondecode(base64decode(var.additional_node_groups_config)), [])
  additional_build_groups = try(jsondecode(var.additional_build_groups_config), jsondecode(base64decode(var.additional_build_groups_config)), [])
}

data "azurerm_client_config" "current" {}

resource "azurerm_resource_group" "rack" {
  name     = local.name
  location = var.region
  tags     = local.tags
}

module "cluster" {
  source = "../../cluster/azure"

  providers = {
    azurerm = azurerm
  }

  additional_node_groups              = local.additional_node_groups
  additional_build_groups             = local.additional_build_groups
  k8s_version                         = var.k8s_version
  max_on_demand_count                 = var.max_on_demand_count
  min_on_demand_count                 = var.min_on_demand_count
  name                                = local.name
  node_disk                           = var.node_disk
  node_type                           = var.node_type
  nvidia_device_plugin_enable         = var.nvidia_device_plugin_enable
  nvidia_device_time_slicing_replicas = var.nvidia_device_time_slicing_replicas
  region                              = var.region
  resource_group                      = azurerm_resource_group.rack.id
  resource_group_name                 = azurerm_resource_group.rack.name
  resource_group_location             = azurerm_resource_group.rack.location
}

module "rack" {
  source = "../../rack/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  cluster                              = module.cluster.id
  docker_hub_username                  = var.docker_hub_username
  docker_hub_password                  = var.docker_hub_password
  high_availability                    = var.high_availability
  idle_timeout                         = var.idle_timeout
  image                                = var.image
  name                                 = local.name
  nginx_additional_config              = var.nginx_additional_config
  nginx_image                          = var.nginx_image
  pdb_default_min_available_percentage = var.pdb_default_min_available_percentage
  rack_name                            = local.rack_name
  region                               = var.region
  release                              = local.release
  resource_group                       = azurerm_resource_group.rack.id
  resource_group_name                  = azurerm_resource_group.rack.name
  resource_group_location              = azurerm_resource_group.rack.location
  ssl_ciphers                          = var.ssl_ciphers
  ssl_protocols                        = var.ssl_protocols
  fluentd_memory                       = var.fluentd_memory
  syslog                               = var.syslog
  tags                                 = local.tags
  telemetry                            = var.telemetry
  telemetry_map                        = local.telemetry_map
  telemetry_default_map                = local.telemetry_default_map
  whitelist                            = split(",", var.whitelist)
  workspace                            = module.cluster.workspace
}
