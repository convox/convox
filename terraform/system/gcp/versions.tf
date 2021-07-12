terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.74.0"
      configuration_aliases = [ google.direct ]
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.74.0"
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
