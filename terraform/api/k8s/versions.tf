terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.2"
    }
  }
  required_version = ">= 0.12"
}
