resource "azuread_application" "api" {
  name                       = "api"
  available_to_other_tenants = false
  oauth2_allow_implicit_flow = true
}

resource "azuread_service_principal" "api" {
  application_id               = azuread_application.api.application_id
  app_role_assignment_required = false
}

resource "random_string" "api_password" {
  length  = 30
  special = true
  upper   = true
}

resource "azuread_service_principal_password" "api" {
  service_principal_id = azuread_service_principal.api.id
  value                = random_string.api_password.result
  end_date             = "2099-01-01T00:00:00Z"
}

resource "azurerm_role_assignment" "principal_api_contributor" {
  scope                = var.resource_group
  role_definition_name = "Contributor"
  principal_id         = azuread_service_principal.api.id
}
