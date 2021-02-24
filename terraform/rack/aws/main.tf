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
  resolver  = module.resolver.endpoint
  router    = module.router.endpoint
}

module "metrics" {
  source = "../../metrics/k8s"

  providers = {
    kubernetes = kubernetes
  }
}

module "resolver" {
  source = "../../resolver/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  namespace = module.k8s.namespace
  rack      = var.name
  release   = var.release
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
  whitelist = var.whitelist
}
