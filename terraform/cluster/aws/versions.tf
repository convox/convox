terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "4.67.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.35.1"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.1"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.1"
    }
    random = {
      source = "hashicorp/random"
    }
  }
  required_version = ">= 1.4"
}
