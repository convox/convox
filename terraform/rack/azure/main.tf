terraform {
  required_version = ">= 0.12.0"
}

provider "azurerm" {
  varsion = "~> 1.37"
}

provider "kubernetes" {
  version = "~> 1.10"
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
  resolver       = module.router.resolver
  resource_group = var.resource_group
  router         = module.router.endpoint
  workspace      = var.workspace
}

module "redis" {
  source = "../../redis/k8s"

  providers = {
    kubernetes = kubernetes
  }

  name      = "redis"
  namespace = module.k8s.namespace
}

module "router" {
  source = "../../router/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  region    = var.region
  release   = var.release

  env = {
    CACHE      = "redis"
    REDIS_ADDR = module.redis.addr
    STORAGE    = "redis"
  }
}
