terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 2.49"
    }
    external = {
      source  = "hashicorp/external"
      version = "~> 1.2"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
