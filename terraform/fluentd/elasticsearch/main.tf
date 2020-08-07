terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.11"
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  cluster   = var.cluster
  image     = "convox/fluentd:1.7"
  namespace = var.namespace
  rack      = var.rack

  target = templatefile("${path.module}/target.conf.tpl", {
    elasticsearch = var.elasticsearch,
    rack          = var.rack,
    syslog        = compact(split(",", var.syslog)),
  })
}
