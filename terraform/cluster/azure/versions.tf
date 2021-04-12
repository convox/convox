terraform {
  required_providers {
    azuread = {
      source = "hashicorp/azuread"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.52"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 1.3"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.2"
    }
  }
  required_version = ">= 0.12"
}
