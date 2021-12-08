terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.3.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
