provider "azurerm" {
  version = "~> 1.36"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.9"

  config_path = module.cluster.kubeconfig
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

  app_id         = data.azurerm_client_config.current.client_id
  name           = var.name
  node_type      = var.node_type
  region         = var.region
  resource_group = azurerm_resource_group.rack.name
  tenant         = data.azurerm_client_config.current.tenant_id
}

# module "identity" {
#   source = "./identity"

#   providers = {
#     kubernetes = kubernetes
#   }

#   kubeconfig = module.cluster.kubeconfig
# }

module "rack" {
  source = "../../rack/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  # identity       = module.identity.id
  kubeconfig     = module.cluster.kubeconfig
  name           = var.name
  region         = var.region
  release        = local.release
  resource_group = azurerm_resource_group.rack.name
  workspace      = module.cluster.workspace
}
