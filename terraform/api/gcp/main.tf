terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.12"
}

provider "kubernetes" {
  version = "~> 1.8"
}

data "google_client_config" "current" {}

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

  domain    = var.domain
  name      = var.name
  namespace = var.namespace
  release   = var.release

  annotations = {
    "cloud.google.com/service-account" : google_service_account.api.email,
    "iam.gke.io/gcp-service-account" : google_service_account.api.email,
  }

  env = {
    BUCKET   = google_storage_bucket.storage.name
    KEY      = google_service_account_key.api.private_key
    PROJECT  = data.google_client_config.current.project,
    PROVIDER = "gcp"
    REGION   = data.google_client_config.current.region
    REGISTRY = data.google_container_registry_repository.registry.repository_url
    ROUTER   = var.router
    SOCKET   = "/var/run/docker.sock"
  }
}
