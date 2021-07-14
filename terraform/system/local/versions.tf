terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
    }
  }
  required_version = ">= 0.12"
}
