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
    http = {
      source  = "hashicorp/http"
      version = "~> 1.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
