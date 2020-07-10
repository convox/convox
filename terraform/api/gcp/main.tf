terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 3.5.0"
}

provider "kubernetes" {
  version = "~> 1.11"
}

data "google_client_config" "current" {}

locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

module "elasticsearch" {
  source = "../../elasticsearch/k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
}

module "fluentd" {
  source = "../../fluentd/elasticsearch"

  providers = {
    kubernetes = kubernetes
  }

  cluster       = var.cluster
  elasticsearch = module.elasticsearch.host
  namespace     = var.namespace
  rack          = var.name
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain    = var.domain
  namespace = var.namespace
  rack      = var.name
  release   = var.release

  annotations = {
    "cloud.google.com/service-account" = google_service_account.api.email
    "iam.gke.io/gcp-service-account"   = google_service_account.api.email
    "kubernetes.io/ingress.class"      = "nginx"
  }

  env = {
    BUCKET       = google_storage_bucket.storage.name
    CERT_MANAGER = "true"
    ELASTIC_URL  = module.elasticsearch.url
    KEY          = google_service_account_key.api.private_key
    PROJECT      = data.google_client_config.current.project,
    PROVIDER     = "gcp"
    REGION       = data.google_client_config.current.region
    REGISTRY     = data.google_container_registry_repository.registry.repository_url
    RESOLVER     = var.resolver
    ROUTER       = var.router
    SOCKET       = "/var/run/docker.sock"
  }
}
