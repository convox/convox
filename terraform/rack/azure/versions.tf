terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.92.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 1.11"
    }
  }
  required_version = ">= 0.12"
}
