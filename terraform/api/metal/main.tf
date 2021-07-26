locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

resource "helm_release" "loki" {
  name       = "loki"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "loki-stack"
  namespace  = var.namespace

  set {
    name  = "loki.persistence.enabled"
    value = "true"
  }

  set {
    name  = "loki.persistence.size"
    value = "1Gi"
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain    = var.domain
  image     = var.image
  namespace = var.namespace
  rack      = var.name
  release   = var.release
  resolver  = var.resolver

  annotations = {
    "cert-manager.io/cluster-issuer" = "self-signed"
    "kubernetes.io/ingress.class"    = "nginx"
  }

  env = {
    CERT_MANAGER = "true"
    LOKI_URL     = "http://loki.${var.namespace}.svc.cluster.local:3100"
    PROVIDER     = "metal"
    REGISTRY     = "registry.${var.domain}"
    RESOLVER     = var.resolver
    ROUTER       = var.router
    SECRET       = var.secret
  }

  volumes = {
    storage = "/var/storage"
  }
}
