terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "4.22.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.10.0"
    }
  }
  required_version = ">= 0.12"
}
