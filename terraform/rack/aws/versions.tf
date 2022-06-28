terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "3.33"
    }
    external = {
      source  = "hashicorp/external"
      version = "~> 2.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.9.0"
    }
  }
  required_version = ">= 0.12"
}
