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
  source = "../../api/oci"

  providers = {
    oci        = oci
    kubernetes = kubernetes
  }

  buildkit_enabled          = var.buildkit_enabled
  cluster                   = var.cluster
  compartment_ocid          = var.compartment_ocid
  docker_hub_authentication = module.k8s.docker_hub_authentication
  fluentd_memory            = var.fluentd_memory
  domain                    = module.router.endpoint
  high_availability         = var.high_availability
  image                     = var.image
  name                      = var.name
  rack_name                 = var.rack_name
  namespace                 = module.k8s.namespace
  region                    = var.region
  release                   = var.release
  resolver                  = module.resolver.endpoint
  router                    = module.router.endpoint
  syslog                    = var.syslog
  tenancy_ocid              = var.tenancy_ocid
  webhook_signing_key       = var.webhook_signing_key
}

module "resolver" {
  source = "../../resolver/oci"

  providers = {
    oci        = oci
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
  source = "../../router/oci"

  providers = {
    oci        = oci
    kubernetes = kubernetes
  }

  docker_hub_authentication = module.k8s.docker_hub_authentication
  high_availability         = var.high_availability
  name                      = var.name
  namespace                 = module.k8s.namespace
  region                    = var.region
  release                   = var.release
  whitelist                 = var.whitelist
}
