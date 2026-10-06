terraform {
  required_version = ">= 0.12"
  required_providers {
    helm = {
      source  = "hashicorp/helm"
      version = "2.12.1"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.5"
    }
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }
}
