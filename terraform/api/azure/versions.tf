terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 0.7"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.52"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.9.0"
    }
    random = {
      source = "hashicorp/random"
    }
    template = {
      source  = "hashicorp/template"
      version = "~> 2.1"
    }
  }
  required_version = ">= 0.12"
}
