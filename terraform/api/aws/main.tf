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

  domain    = var.domain
  namespace = var.namespace
  rack      = var.name
  release   = var.release

  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
    "eks.amazonaws.com/role-arn"     = aws_iam_role.api.arn
    "iam.amazonaws.com/role"         = aws_iam_role.api.arn
    "kubernetes.io/ingress.class"    = "nginx"
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
