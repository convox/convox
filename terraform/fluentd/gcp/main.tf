terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.12"
}

provider "kubernetes" {
  version = "~> 1.10"
}

locals {
  tags = {
    System  = "convox"
    Cluster = var.cluster
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  cluster   = var.cluster
  image     = "fluent/fluentd-kubernetes-daemonset:v1.3.1-debian-stackdriver-1.3"
  namespace = var.namespace
  rack      = var.rack
  target    = file("${path.module}/target.conf")

  annotations = {
    "cloud.google.com/service-account" : google_service_account.fluentd.email,
    "iam.gke.io/gcp-service-account" : google_service_account.fluentd.email,
  }
}
