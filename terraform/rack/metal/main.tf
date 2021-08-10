locals {
  endpoint = coalesce(var.domain, module.router.endpoint)
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  domain  = module.router.endpoint
  name    = var.name
  release = var.release
}

module "api" {
  source = "../../api/metal"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  domain    = local.endpoint
  image     = var.image
  name      = var.name
  namespace = module.k8s.namespace
  release   = var.release
  resolver  = module.resolver.endpoint
  router    = module.router.endpoint
  secret    = random_string.secret.result
  syslog    = var.syslog
}

module "resolver" {
  source = "../../resolver/metal"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  image     = var.image
  namespace = module.k8s.namespace
  rack      = var.name
  release   = var.release
}

module "router" {
  source = "../../router/metal"

  providers = {
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  release   = var.release
  whitelist = var.whitelist
}
