terraform {
  required_version = ">= 0.12"
  required_providers {
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
