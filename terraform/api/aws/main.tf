terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.33"
}

provider "kubernetes" {
  version = "1.9"
}

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

  domain     = var.domain
  kubeconfig = var.kubeconfig
  name       = var.name
  namespace  = var.namespace
  release    = var.release

  annotations = {
    "eks.amazonaws.com/role-arn" : aws_iam_role.api.arn,
    "iam.amazonaws.com/role" : aws_iam_role.api.arn,
  }

  env = {
    AWS_REGION = data.aws_region.current.name
    BUCKET     = aws_s3_bucket.storage.id
    PROVIDER   = "aws"
    ROUTER     = var.router
    SOCKET     = "/var/run/docker.sock"
  }
}
