terraform {
  required_version = ">= 0.12"
  required_providers {
    http = {
      source  = "hashicorp/http"
      version = "~> 2.1"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.35.1"
    }
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }
}
