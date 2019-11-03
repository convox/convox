terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.9"
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  cluster   = var.cluster
  image     = "fluent/fluentd-kubernetes-daemonset:v1.7-debian-elasticsearch6-1"
  namespace = var.namespace
  target    = file("${path.module}/target.conf")
}
