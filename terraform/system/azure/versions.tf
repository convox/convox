terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    http = {
      source = "hashicorp/http"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
  required_version = ">= 0.13"
}
