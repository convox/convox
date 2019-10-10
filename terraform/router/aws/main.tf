terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "kubernetes" {
  version = "~> 1.8"
}

locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

data "aws_region" "current" {
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
  release   = var.release

  annotations = {
    "iam.amazonaws.com/role" : aws_iam_role.router.arn,
  }

  env = {
    AWS_REGION     = data.aws_region.current.name
    CACHE          = "dynamodb"
    STORAGE        = "dynamodb"
    ROUTER_CACHE   = aws_dynamodb_table.cache.name
    ROUTER_HOSTS   = aws_dynamodb_table.hosts.name
    ROUTER_TARGETS = aws_dynamodb_table.targets.name
  }
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
  }

  spec {
    external_traffic_policy = "Local"
    type                    = "NodePort"

    port {
      name        = "http"
      node_port   = 32000
      port        = 80
      protocol    = "TCP"
      target_port = 80
    }

    port {
      name        = "https"
      node_port   = 32001
      port        = 443
      protocol    = "TCP"
      target_port = 443
    }

    selector = {
      system  = "convox"
      service = "router"
    }
  }
}
