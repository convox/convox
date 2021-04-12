terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
      version = "~> 1.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 2.1"
    }
  }
  required_version = ">= 0.12"
}
