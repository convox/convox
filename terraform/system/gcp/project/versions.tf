terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.69.1"
    }
  }
  required_version = ">= 1.4"
}
