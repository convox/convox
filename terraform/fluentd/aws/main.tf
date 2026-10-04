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

  fluentd_disable = var.fluentd_disable
  fluentd_memory  = var.fluentd_memory

  cluster   = var.cluster
  image     = "convox/fluentd:1.19-all@sha256:bb519f1ca5b8d28a7c94c9dfd52af2c8befa55ed941daadaca90f6d4c725fd45"
  namespace = var.namespace
  rack      = var.rack

  target = templatefile("${path.module}/target.conf.tpl", {
    access_log_retention   = var.access_log_retention_in_days,
    app_cloudwatch_disable = var.app_cloudwatch_disable,
    rack                   = var.rack,
    region                 = data.aws_region.current.name,
    syslog                 = compact(split(",", var.syslog))
  })

  annotations = {
    "eks.amazonaws.com/role-arn" = aws_iam_role.fluentd.arn,
    "iam.amazonaws.com/role"     = aws_iam_role.fluentd.arn,
    "convox.com/dummy"           = var.access_log_retention_in_days,
  }

  env = {
    AWS_REGION = data.aws_region.current.name
  }
}
