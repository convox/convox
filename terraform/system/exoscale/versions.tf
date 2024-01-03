terraform {
  required_version = ">= 0.12"
  required_providers {
    exoscale = {
      source  = "exoscale/exoscale"
      version = "~> 0.54"
    }

    http = {
      source  = "hashicorp/http"
      version = "~> 2.1"
    }

    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.19.0"
    }
  }
}
