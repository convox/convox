terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
      version = "~> 1.1"
    }
    helm = {
      source = "hashicorp/helm"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
