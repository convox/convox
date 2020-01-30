provider "google" {
  version = "~> 3.5.0"
}

provider "kubernetes" {
  version = "~> 1.10"
}

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

  domain        = module.router.endpoint
  name          = var.name
  namespace     = module.k8s.namespace
  nodes_account = var.nodes_account
  release       = var.release
  resolver      = module.router.resolver
  router        = module.router.endpoint
}

module "redis" {
  source = "../../redis/gcp"

  providers = {
    google = google
  }

  name    = var.name
  network = var.network
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

  env = {
    CACHE      = "redis"
    REDIS_ADDR = module.redis.addr
    STORAGE    = "redis"
  }
}
