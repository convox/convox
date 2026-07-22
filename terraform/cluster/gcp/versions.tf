terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.31.0"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "2.12.1"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.5"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.1"
    }
  }
  required_version = ">= 0.12"
}
