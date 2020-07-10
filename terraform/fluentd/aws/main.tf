terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.49"
}

provider "kubernetes" {
  version = "~> 1.11"
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
  image     = "fluent/fluentd-kubernetes-daemonset:v1.7.3-debian-cloudwatch-1.0"
  namespace = var.namespace
  rack      = var.rack
  target    = templatefile("${path.module}/target.conf.tpl", { region = data.aws_region.current.name })

  annotations = {
    "eks.amazonaws.com/role-arn" : aws_iam_role.fluentd.arn,
    "iam.amazonaws.com/role" = aws_iam_role.fluentd.arn
  }

  env = {
    AWS_REGION = data.aws_region.current.name
  }
}
