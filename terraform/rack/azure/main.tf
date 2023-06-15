module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  domain              = module.router.endpoint
  name                = var.name
  release             = var.release
  settings            = var.settings
  telemetry           = var.telemetry
}

module "api" {
  source = "../../api/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  cluster                   = var.cluster
  docker_hub_authentication = module.k8s.docker_hub_authentication
  domain                    = module.router.endpoint
  image                     = var.image
  name                      = var.name
  rack_name                 = var.rack_name
  namespace                 = module.k8s.namespace
  region                    = var.region
  release                   = var.release
  resolver                  = module.resolver.endpoint
  resource_group            = var.resource_group
  resource_group_name       = var.resource_group_name
  resource_group_location   = var.resource_group_location
  router                    = module.router.endpoint
  syslog                    = var.syslog
  workspace                 = var.workspace
}

module "resolver" {
  source = "../../resolver/azure"

  providers = {
    azurerm    = azurerm
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  image                     = var.image
  namespace                 = module.k8s.namespace
  rack                      = var.name
  release                   = var.release
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
  whitelist = var.whitelist
}
