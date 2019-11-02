terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.12"
}

# data "aws_caller_identity" "current" {}
# data "aws_region" "current" {}

locals {
  tags = {
    System  = "convox"
    Cluster = var.cluster
  }
}

module "k8s" {
  source = "../k8s"

  cluster    = var.cluster
  image      = "fluent/fluentd-kubernetes-daemonset:v1.3.1-debian-stackdriver-1.3"
  kubeconfig = var.kubeconfig
  namespace  = var.namespace
  target     = file("${path.module}/target.conf")

  annotations = {
    "cloud.google.com/service-account" : google_service_account.fluentd.email,
    "iam.gke.io/gcp-service-account" : google_service_account.fluentd.email,
  }

  # env = {
  #   AWS_REGION = data.aws_region.current.name
  # }
}
