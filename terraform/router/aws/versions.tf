terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
    http = {
      source = "hashicorp/http"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
}
