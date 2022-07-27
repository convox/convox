terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.10.0"
    }
    random = {
      source = "hashicorp/random"
    }
  }
  required_version = ">= 0.12"
}
