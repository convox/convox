locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  authentication = true
  domain         = var.domain
  image          = var.image
  namespace      = var.namespace
  rack           = var.name
  release        = var.release
  replicas       = 1
  resolver       = var.resolver

  annotations = {
    "cert-manager.io/cluster-issuer" = "self-signed"
    "convox.com/idles"               = "true"
    "kubernetes.io/ingress.class"    = "nginx"
  }

  env = {
    CERT_MANAGER = "true"
    PROVIDER     = "local"
    REGISTRY     = "registry.${var.domain}"
    RESOLVER     = var.resolver
    ROUTER       = var.router
    SECRET       = var.secret
    STORAGE      = "/var/storage"
  }

  volumes = {
    storage-local = "/var/storage"
  }
}
