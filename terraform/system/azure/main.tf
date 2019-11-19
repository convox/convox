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

resource "azurerm_resource_group" "rack" {
  name     = var.name
  location = var.region
}

module "cluster" {
  source = "../../cluster/azure"

  providers = {
    azurerm = azurerm
  }

  app_id         = var.app_id
  name           = var.name
  node_type      = var.node_type
  password       = var.password
  region         = var.region
  resource_group = azurerm_resource_group.rack.name
  tenant         = var.tenant
}

# module "elasticsearch" {
#   source = "../../elasticsearch/k8s"

#   providers = {
#     kubernetes = kubernetes
#   }

#   namespace = "kube-system"
# }

# module "fluentd" {
#   source = "../../fluentd/do"

#   providers = {
#     kubernetes = kubernetes
#   }

#   cluster   = var.name
#   namespace = "kube-system"
#   name      = var.name
# }

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
