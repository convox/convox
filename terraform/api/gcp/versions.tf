terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.69.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.19.0"
    }
    random = {
      source = "hashicorp/random"
    }
  }
  required_version = ">= 0.12"
}
