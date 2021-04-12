terraform {
  required_providers {
    external = {
      source  = "hashicorp/external"
      version = "~> 1.2"
    }
  }
  required_version = ">= 0.12"
}
