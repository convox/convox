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
  source = "../../api/gcp"

  providers = {
    google     = google
    kubernetes = kubernetes
  }

  cluster       = var.cluster
  domain        = module.router.endpoint
  image         = var.image
  name          = var.name
  namespace     = module.k8s.namespace
  nodes_account = var.nodes_account
  release       = var.release
  resolver      = module.resolver.endpoint
  router        = module.router.endpoint
  syslog        = var.syslog
}

module "resolver" {
  source = "../../resolver/gcp"

  providers = {
    google     = google
    kubernetes = kubernetes
  }

  image     = var.image
  namespace = module.k8s.namespace
  rack      = var.name
  release   = var.release
}

module "router" {
  source = "../../router/gcp"

  providers = {
    google     = google
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  network   = var.network
  release   = var.release
  whitelist = var.whitelist
}
