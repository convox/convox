terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.8"

  config_path = var.kubeconfig
}

module "atom" {
  source = "../atom/k8s"

  providers = {
    kubernetes = kubernetes
  }

  kubeconfig = var.kubeconfig
  namespace  = var.namespace
}

module "router" {
  source = "../router/k8s"

  providers = {
    kubernetes = kubernetes
  }

  annotations = var.router_annotations
  env         = var.router_env
  namespace   = var.namespace
}
