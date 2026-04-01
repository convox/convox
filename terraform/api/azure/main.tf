locals {
  prefix = format("%.12s", replace(lower(var.name), "/[^a-z0-9]/", ""))
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

data "azurerm_client_config" "current" {}

data "azurerm_subscription" "current" {}

resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

module "elasticsearch" {
  source = "../../elasticsearch/k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
}

module "fluentd" {
  source = "../../fluentd/elasticsearch"

  providers = {
    kubernetes = kubernetes
  }

  cluster        = var.cluster
  elasticsearch  = module.elasticsearch.host
  fluentd_memory = var.fluentd_memory
  namespace      = var.namespace
  rack           = var.name
  syslog         = var.syslog
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = var.docker_hub_authentication
  domain                    = var.domain
  domain_internal           = var.domain_internal
  image                     = var.image
  namespace                 = var.namespace
  rack                      = var.name
  rack_name                 = var.rack_name
  release                   = var.release
  replicas                  = var.high_availability ? 2 : 1
  resolver                  = var.resolver

  labels = {
    "aadpodidbinding" : "api"
  }

  env = {
    AZURE_CLIENT_ID                      = azuread_service_principal.api.client_id
    AZURE_CLIENT_SECRET                  = azuread_service_principal_password.api.value
    AZURE_SUBSCRIPTION_ID                = data.azurerm_subscription.current.subscription_id
    AZURE_TENANT_ID                      = data.azurerm_client_config.current.tenant_id
    CERT_MANAGER                         = "true"
    ELASTIC_URL                          = module.elasticsearch.url
    PDB_DEFAULT_MIN_AVAILABLE_PERCENTAGE = var.pdb_default_min_available_percentage
    PROVIDER                             = "azure"
    REGION                               = var.region
    REGISTRY                             = azurerm_container_registry.registry.login_server
    RESOLVER                             = var.resolver
    RESOURCE_GROUP                       = var.name
    ROUTER                               = var.router
    STORAGE_ACCOUNT                      = azurerm_storage_account.storage.name
    STORAGE_SHARE                        = azurerm_storage_share.storage.name
    WORKSPACE                            = var.workspace
  }
}
