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
  source = "../../api/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  domain                    = module.router.endpoint
  high_availability         = var.high_availability
  image                     = var.image
  name                      = var.name
  namespace                 = module.k8s.namespace
  oidc_arn                  = var.oidc_arn
  oidc_sub                  = var.oidc_sub
  release                   = var.release
  resolver                  = module.resolver.endpoint
  router                    = module.router.endpoint
}

module "metrics" {
  source = "../../metrics/k8s"

  providers = {
    kubernetes = kubernetes
  }

}

module "resolver" {
  source = "../../resolver/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  high_availability         = var.high_availability
  image                     = var.image
  namespace                 = module.k8s.namespace
  rack                      = var.name
  release                   = var.release
}

module "router" {
  source = "../../router/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  high_availability = var.high_availability
  idle_timeout      = var.idle_timeout
  name              = var.name
  namespace         = module.k8s.namespace
  oidc_arn          = var.oidc_arn
  oidc_sub          = var.oidc_sub
  release           = var.release
  whitelist         = var.whitelist
}
