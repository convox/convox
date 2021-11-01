terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.5.0"
    }
  }
  required_version = ">= 0.12"
}
