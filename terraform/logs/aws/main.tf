terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "kubernetes" {
  version = "~> 1.8"
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

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
  image     = "fluent/fluentd-kubernetes-daemonset:v1.3.3-debian-cloudwatch-1.4"
  namespace = var.namespace
  target    = file("${path.module}/target.conf")

  annotations = {
    "iam.amazonaws.com/role" = aws_iam_role.logs.arn
  }

  env = {
    AWS_REGION = data.aws_region.current.name
  }
}
