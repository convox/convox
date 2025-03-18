terraform {
  required_providers {
    external = {
      source  = "hashicorp/external"
      version = "~> 2.1"
    }
  }
  required_version = ">= 1.4"
}
