terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.31.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.35.1"
    }
    random = {
      source = "hashicorp/random"
    }
  }
  required_version = ">= 0.12"
}
