terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
    }
    tls = {
      source  = "hashicorp/tls"
    }
  }
  required_version = ">= 0.12"
}
