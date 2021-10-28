terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
