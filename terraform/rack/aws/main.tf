terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "external" {
  version = "~> 1.2"
}

provider "kubernetes" {
  version = "~> 1.10"
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain  = module.router.endpoint
  name    = var.name
  release = var.release
}

module "api" {
  source = "../../api/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  domain    = module.router.endpoint
  name      = var.name
  namespace = module.k8s.namespace
  oidc_arn  = var.oidc_arn
  oidc_sub  = var.oidc_sub
  release   = var.release
  router    = module.router.endpoint
}

module "router" {
  source = "../../router/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  oidc_arn  = var.oidc_arn
  oidc_sub  = var.oidc_sub
  release   = var.release
}
