module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username   = var.docker_hub_username
  docker_hub_password   = var.docker_hub_password
  domain                = module.router.endpoint
  eks_addons            = var.eks_addons
  name                  = var.name
  release               = var.release
  telemetry             = var.telemetry
  telemetry_map         = var.telemetry_map
  telemetry_default_map = var.telemetry_default_map
}

module "api" {
  source = "../../api/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  buildkit_enabled                     = var.buildkit_enabled
  build_disable_convox_resolver        = var.build_disable_convox_resolver
  build_node_enabled                   = var.build_node_enabled
  convox_domain_tls_cert_disable       = var.convox_domain_tls_cert_disable
  docker_hub_authentication            = module.k8s.docker_hub_authentication
  docker_hub_username                  = var.docker_hub_username
  docker_hub_password                  = var.docker_hub_password
  domain                               = try(module.router.endpoint, "") # terraform destroy sometimes failes to resolve the value
  domain_internal                      = module.router.endpoint_internal
  disable_image_manifest_cache         = var.disable_image_manifest_cache
  ecr_scan_on_push_enable              = var.ecr_scan_on_push_enable
  efs_csi_driver_enable                = var.efs_csi_driver_enable
  efs_file_system_id                   = var.efs_file_system_id
  high_availability                    = var.high_availability
  metrics_scraper_host                 = module.metrics.metrics_scraper_host
  image                                = var.image
  name                                 = var.name
  rack_name                            = var.rack_name
  namespace                            = module.k8s.namespace
  oidc_arn                             = var.oidc_arn
  oidc_sub                             = var.oidc_sub
  pdb_default_min_available_percentage = var.pdb_default_min_available_percentage
  release                              = var.release
  resolver                             = module.resolver.endpoint
  router                               = module.router.endpoint
  subnets                              = var.subnets
  vpc_id                               = var.vpc_id
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

  convox_rack_domain        = var.convox_rack_domain
  deploy_extra_nlb          = var.deploy_extra_nlb
  docker_hub_authentication = module.k8s.docker_hub_authentication
  high_availability         = var.high_availability
  idle_timeout              = var.idle_timeout
  internal_router           = var.internal_router
  name                      = var.name
  namespace                 = module.k8s.namespace
  nlb_security_group        = var.nlb_security_group
  oidc_arn                  = var.oidc_arn
  oidc_sub                  = var.oidc_sub
  proxy_protocol            = var.proxy_protocol
  release                   = var.release
  ssl_ciphers               = var.ssl_ciphers
  ssl_protocols             = var.ssl_protocols
  tags                      = var.tags
  whitelist                 = var.whitelist
  lbc_helm_id               = var.lbc_helm_id
}
