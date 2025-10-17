module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username   = var.docker_hub_username
  docker_hub_password   = var.docker_hub_password
  domain                = module.router.endpoint
  name                  = var.name
  release               = var.release
  telemetry             = var.telemetry
  telemetry_map         = var.telemetry_map
  telemetry_default_map = var.telemetry_default_map
}

module "api" {
  source = "../../api/local"

  providers = {
    kubernetes = kubernetes
  }

  domain                    = module.router.endpoint
  docker_hub_authentication = module.k8s.docker_hub_authentication
  image                     = var.image
  name                      = var.name
  rack_name                 = var.rack_name
  namespace                 = module.k8s.namespace
  release                   = var.release
  resolver                  = module.resolver.endpoint
  router                    = module.router.endpoint
  secret                    = random_string.secret.result
  private_api               = var.private_api
}

module "resolver" {
  source = "../../resolver/local"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  image                     = var.image
  namespace                 = module.k8s.namespace
  platform                  = var.platform
  rack                      = var.name
  release                   = var.release
}

module "router" {
  source    = "../../router/local"
  name      = var.name
  namespace = module.k8s.namespace
  os        = var.os
}
