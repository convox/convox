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
  image     = "convox/fluentd:1.7.1"
  namespace = var.namespace
  rack      = var.rack

  target = templatefile("${path.module}/target.conf.tpl", {
    rack   = var.rack,
    region = data.aws_region.current.name,
    syslog = compact(split(",", var.syslog)),
  })

  annotations = {
    "eks.amazonaws.com/role-arn" = aws_iam_role.fluentd.arn,
    "iam.amazonaws.com/role"     = aws_iam_role.fluentd.arn
  }

  env = {
    AWS_REGION = data.aws_region.current.name
  }
}
