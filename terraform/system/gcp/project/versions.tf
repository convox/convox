terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.74.0"
    }
  }
  required_version = ">= 0.12"
}
