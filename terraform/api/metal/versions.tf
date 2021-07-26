terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
    helm = {
      source = "hashicorp/helm"
    }
  }
  required_version = ">= 0.12"
}
