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
  source = "../../api/exoscale"

  providers = {
    aws        = aws
    exoscale   = exoscale
    kubernetes = kubernetes
  }

  buildkit_enabled             = var.buildkit_enabled
  build_node_enabled           = var.build_node_enabled
  docker_hub_authentication    = module.k8s.docker_hub_authentication
  domain                       = try(module.router.endpoint, "") # terraform destroy sometimes failes to resolve the value
  domain_internal              = module.router.endpoint_internal
  disable_image_manifest_cache = var.disable_image_manifest_cache
  high_availability            = var.high_availability
  metrics_scraper_host         = module.metrics.metrics_scraper_host
  image                        = var.image
  name                         = var.name
  rack_name                    = var.rack_name
  namespace                    = module.k8s.namespace
  release                      = var.release
  resolver                     = module.resolver.endpoint
  router                       = module.router.endpoint
  zone = var.zone
}

module "metrics" {
  source = "../../metrics/k8s"

  providers = {
    kubernetes = kubernetes
  }

}

module "resolver" {
  source = "../../resolver/exoscale"

  providers = {
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
