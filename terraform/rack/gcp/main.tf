
terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.12"

  credentials = pathexpand(var.credentials)
  project     = var.project
}

provider "kubernetes" {
  version = "~> 1.9"

  config_path = var.kubeconfig
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain     = var.domain
  kubeconfig = var.kubeconfig
  name       = var.name
  release    = var.release
}

module "api" {
  source = "../../api/gcp"

  providers = {
    google     = google
    kubernetes = kubernetes
  }

  domain        = var.domain
  kubeconfig    = var.kubeconfig
  name          = var.name
  namespace     = module.k8s.namespace
  nodes_account = var.nodes_account
  release       = var.release
  router        = module.router.endpoint
}

module "router" {
  source = "../../router/gcp"

  providers = {
    google     = google
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  release   = var.release
}
