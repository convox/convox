provider "azurerm" {
  version = "~> 1.37"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.9"

  client_certificate     = module.cluster.client_certificate
  client_key             = module.cluster.client_key
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

  load_config_file = false
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases"
}

locals {
  current = jsondecode(data.http.releases.body).0.tag_name
  release = coalesce(var.release, local.current)
}

data "azurerm_client_config" "current" {}

resource "azurerm_resource_group" "rack" {
  name     = var.name
  location = var.region
}

module "cluster" {
  source = "../../cluster/azure"

  providers = {
    azurerm = azurerm
  }

  name           = var.name
  node_type      = var.node_type
  region         = var.region
  resource_group = azurerm_resource_group.rack.name
}

module "rack" {
  source = "../../rack/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  name           = var.name
  region         = var.region
  release        = local.release
  resource_group = azurerm_resource_group.rack.name
  workspace      = module.cluster.workspace
}
