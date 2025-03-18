terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.69.1"
    }
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
