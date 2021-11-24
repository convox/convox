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
}

module "api" {
  source = "../../api/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  access_id                 = var.access_id
  cluster                   = var.cluster
  docker_hub_authentication = module.k8s.docker_hub_authentication
  domain                    = module.router.endpoint
  high_availability         = var.high_availability
  image                     = var.image
  name                      = var.name
  namespace                 = module.k8s.namespace
  region                    = var.region
  release                   = var.release
  resolver                  = module.resolver.endpoint
  router                    = module.router.endpoint
  secret                    = random_string.secret.result
  secret_key                = var.secret_key
  syslog                    = var.syslog
}

module "resolver" {
  source = "../../resolver/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  high_availability         = var.high_availability
  image                     = var.image
  namespace                 = module.k8s.namespace
  rack                      = var.name
  release                   = var.release
}

module "router" {
  source = "../../router/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  high_availability = var.high_availability
  name              = var.name
  namespace         = module.k8s.namespace
  region            = var.region
  release           = var.release
  whitelist         = var.whitelist
}
