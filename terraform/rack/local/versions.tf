terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
    random = {
      source = "hashicorp/random"
    }
  }
  required_version = ">= 0.12"
}
