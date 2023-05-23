data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

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

  buildkit_enabled          = var.buildkit_enabled
  build_node_enabled        = var.build_node_enabled
  docker_hub_authentication = var.docker_hub_authentication
  domain                    = var.domain
  domain_internal           = var.domain_internal
  image                     = var.image
  metrics_scraper_host      = var.metrics_scraper_host
  namespace                 = var.namespace
  rack                      = var.name
  rack_name                 = var.rack_name
  release                   = var.release
  replicas                  = var.high_availability ? 2 : 1
  resolver                  = var.resolver

  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
    "cert-manager.io/duration"       = var.cert_duration
    "eks.amazonaws.com/role-arn"     = aws_iam_role.api.arn
    "iam.amazonaws.com/role"         = aws_iam_role.api.arn
  }

  env = {
    AWS_REGION   = data.aws_region.current.name
    BUCKET       = aws_s3_bucket.storage.id
    CERT_MANAGER = "true"
    PROVIDER     = "aws"
    RESOLVER     = var.resolver
    ROUTER       = var.router
    SOCKET       = "/var/run/docker.sock"
  }
}
