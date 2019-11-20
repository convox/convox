terraform {
  required_version = ">= 0.12.0"
}

provider "azurerm" {
  varsion = "~> 1.36"
}

provider "kubernetes" {
  version = "~> 1.9"
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain  = module.router.endpoint
  name    = var.name
  release = var.release
}

module "api" {
  source = "../../api/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  domain         = module.router.endpoint
  name           = var.name
  namespace      = module.k8s.namespace
  region         = var.region
  release        = var.release
  resource_group = var.resource_group
  router         = module.router.endpoint
  # secret     = random_string.secret.result
  workspace = var.workspace
}

module "router" {
  source = "../../router/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  name           = var.name
  namespace      = module.k8s.namespace
  region         = var.region
  release        = var.release
  resource_group = var.resource_group
}
