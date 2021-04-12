terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.5.0"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 1.3"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.2"
    }
  }
  required_version = ">= 0.12"
}
