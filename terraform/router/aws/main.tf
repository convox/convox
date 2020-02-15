terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10"
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
  rack      = var.name
  release   = var.release

  annotations = {
    "eks.amazonaws.com/role-arn" : aws_iam_role.router.arn,
    "iam.amazonaws.com/role" : aws_iam_role.router.arn,
  }

  env = merge(var.env, {
    AUTOCERT        = "true"
    AWS_REGION      = data.aws_region.current.name
    CACHE           = "dynamodb"
    DYNAMODB_CACHE  = aws_dynamodb_table.cache.name
    DYNAMODB_HOSTS  = aws_dynamodb_table.hosts.name
    DYNAMODB_ROUTES = aws_dynamodb_table.routes.name
    STORAGE         = "dynamodb"
  })
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
  }

  spec {
    type = "LoadBalancer"

    port {
      name        = "http"
      port        = 80
      protocol    = "TCP"
      target_port = 80
    }

    port {
      name        = "https"
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

data "http" "alias" {
  url = "https://alias.convox.com/alias/${kubernetes_service.router.load_balancer_ingress.0.hostname}"
}
