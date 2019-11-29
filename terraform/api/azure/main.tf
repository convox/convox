terraform {
  required_version = ">= 0.12.0"
}

provider "azuread" {
  version = "~> 0.7"
}

provider "azurerm" {
  version = "~> 1.37"
}

provider "kubernetes" {
  version = "~> 1.10"
}

provider "template" {
  version = "~> 2.1"
}

locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

data "azurerm_client_config" "current" {}

data "azurerm_resource_group" "rack" {
  name = var.resource_group
}

data "azurerm_subscription" "current" {}

resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain    = var.domain
  name      = var.name
  namespace = var.namespace
  release   = var.release

  annotations = {}

  labels = {
    "aadpodidbinding" : "api"
  }

  env = {
    AZURE_CLIENT_ID       = azuread_service_principal.api.application_id
    AZURE_CLIENT_SECRET   = azuread_service_principal_password.api.value
    AZURE_SUBSCRIPTION_ID = data.azurerm_subscription.current.subscription_id
    AZURE_TENANT_ID       = data.azurerm_client_config.current.tenant_id
    PROVIDER              = "azure"
    REGION                = var.region
    REGISTRY              = azurerm_container_registry.registry.login_server
    RESOURCE_GROUP        = var.resource_group
    ROUTER                = var.router
    STORAGE_ACCOUNT       = azurerm_storage_account.storage.name
    STORAGE_SHARE         = azurerm_storage_share.storage.name
    WORKSPACE             = var.workspace
  }
}
