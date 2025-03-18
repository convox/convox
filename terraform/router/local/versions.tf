terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
      version = "~> 2.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.35.1"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 3.1"
    }
  }
  required_version = ">= 1.4"
}
