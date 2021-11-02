terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "3.33"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
