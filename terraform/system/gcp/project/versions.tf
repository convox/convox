terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.3.0"
    }
  }
  required_version = ">= 0.12"
}
