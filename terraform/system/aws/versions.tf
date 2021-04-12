terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 2.49"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 1.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
