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
}

data "azurerm_client_config" "current" {}

resource "azurerm_resource_group" "rack" {
  name     = local.name
  location = var.region
}

module "cluster" {
  source = "../../cluster/azure"

  providers = {
    azurerm = azurerm
  }

  k8s_version             = var.k8s_version
  name                    = local.name
  node_type               = var.node_type
  region                  = var.region
  resource_group          = azurerm_resource_group.rack.id
  resource_group_name     = azurerm_resource_group.rack.name
  resource_group_location = azurerm_resource_group.rack.location
}

module "rack" {
  source = "../../rack/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  cluster                 = module.cluster.id
  docker_hub_username     = var.docker_hub_username
  docker_hub_password     = var.docker_hub_password
  image                   = var.image
  name                    = local.name
  rack_name               = local.rack_name
  region                  = var.region
  release                 = local.release
  resource_group          = azurerm_resource_group.rack.id
  resource_group_name     = azurerm_resource_group.rack.name
  resource_group_location = azurerm_resource_group.rack.location
  syslog                  = var.syslog
  telemetry               = var.telemetry
  telemetry_map           = local.telemetry_map
  telemetry_default_map   = local.telemetry_default_map
  whitelist               = split(",", var.whitelist)
  workspace               = module.cluster.workspace
}
