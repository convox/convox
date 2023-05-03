resource "azuread_application" "cluster" {
  display_name = "cluster"
  web {
    implicit_grant {
      access_token_issuance_enabled = true
    }
  }
}

resource "azuread_service_principal" "cluster" {
  application_id               = azuread_application.cluster.application_id
  app_role_assignment_required = false
}

resource "random_string" "cluster_password" {
  length  = 30
  special = true
  upper   = true
}

resource "azuread_service_principal_password" "cluster" {
  service_principal_id = azuread_service_principal.cluster.id
  end_date             = "2099-01-01T00:00:00Z"
}

resource "azurerm_role_assignment" "cluster-contributor" {
  scope                = var.resource_group
  role_definition_name = "Contributor"
  principal_id         = azuread_service_principal.cluster.id
}
