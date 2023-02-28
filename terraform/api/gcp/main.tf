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
  syslog        = var.syslog
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  buildkit_enabled          = var.buildkit_enabled
  docker_hub_authentication = var.docker_hub_authentication
  domain                    = var.domain
  image                     = var.image
  namespace                 = var.namespace
  rack                      = var.name
  release                   = var.release
  resolver                  = var.resolver


  annotations = {
    "cert-manager.io/cluster-issuer"   = "letsencrypt"
    "cloud.google.com/service-account" = google_service_account.api.email
    "iam.gke.io/gcp-service-account"   = google_service_account.api.email
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
