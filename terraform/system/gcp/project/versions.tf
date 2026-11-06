terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.31.0"
    }
  }
  required_version = ">= 0.12"
}
