terraform {
  required_version = ">= 0.12"
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 1.13"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.10.0"
    }
  }
}
