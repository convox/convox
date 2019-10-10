terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.8"

  config_path = var.kubeconfig
}

resource "kubernetes_namespace" "system" {
  metadata {
    labels = {
      rack   = var.name
      system = "convox"
      app    = "system"
    }

    name = "${var.name}-system"
  }
}

resource "kubernetes_config_map" "rack" {
  metadata {
    namespace = kubernetes_namespace.system.metadata.0.name
    name      = "rack"
  }

  data = {
    DOMAIN = var.domain
  }
}

module "atom" {
  source = "../../atom/k8s"

  providers = {
    kubernetes = kubernetes
  }

  kubeconfig = var.kubeconfig
  namespace  = kubernetes_namespace.system.metadata.0.name
  release    = var.release
}
