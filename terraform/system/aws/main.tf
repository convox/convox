terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "kubernetes" {
  version = "~> 1.8"

  config_path = module.cluster.kubeconfig
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases"
}

locals {
  current = jsondecode(data.http.releases.body).0.tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/aws"

  providers = {
    aws = aws
  }

  cidr      = var.cidr
  name      = var.name
  node_type = var.node_type
  ssh_key   = var.ssh_key
}

module "rack" {
  source = "../../rack/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  domain             = var.domain
  kubeconfig         = module.cluster.kubeconfig
  name               = var.name
  nodes_role         = module.cluster.nodes_role
  nodes_security     = module.cluster.nodes_security
  release            = local.release
  subnets_private    = module.cluster.subnets_private
  subnets_public     = module.cluster.subnets_public
  target_group_http  = module.cluster.target_group_http
  target_group_https = module.cluster.target_group_https
}
