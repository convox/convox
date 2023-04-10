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

  buildkit_enabled          = var.buildkit_enabled
  docker_hub_authentication = module.k8s.docker_hub_authentication
  domain                    = try(module.router.endpoint, "") # terraform destroy sometimes failes to resolve the value
  domain_internal           = module.router.endpoint_internal
  high_availability         = var.high_availability
  metrics_scraper_host      = module.metrics.metrics_scraper_host
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
  internal_router   = var.internal_router
  name              = var.name
  namespace         = module.k8s.namespace
  oidc_arn          = var.oidc_arn
  oidc_sub          = var.oidc_sub
  proxy_protocol    = var.proxy_protocol
  release           = var.release
  ssl_ciphers       = var.ssl_ciphers
  ssl_protocols     = var.ssl_protocols
  tags              = var.tags
  whitelist         = var.whitelist
}
