resource "azuread_application" "api" {
  display_name = "api"
  web {
    implicit_grant {
      access_token_issuance_enabled = true
    }
  }
}

resource "azuread_service_principal" "api" {
  client_id                    = azuread_application.api.client_id
  app_role_assignment_required = false
}

resource "azuread_service_principal_password" "api" {
  service_principal_id = azuread_service_principal.api.id
  end_date             = "2099-01-01T00:00:00Z"
}

resource "azurerm_role_assignment" "principal_api_contributor" {
  scope                = var.resource_group
  role_definition_name = "Contributor"
  principal_id         = azuread_service_principal.api.object_id
}
