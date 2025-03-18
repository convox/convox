terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.35.1"
    }
    random = {
      source = "hashicorp/random"
    }
  }
  required_version = ">= 1.4"
}
