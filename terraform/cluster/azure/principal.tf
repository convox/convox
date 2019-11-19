resource "azuread_application" "cluster" {
  name                       = "cluster"
  available_to_other_tenants = false
  oauth2_allow_implicit_flow = true
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
  value                = random_string.cluster_password.result
  end_date             = "2099-01-01T00:00:00Z"
}

resource "azurerm_role_assignment" "cluster-contributor" {
  scope                = data.azurerm_resource_group.system.id
  role_definition_name = "Contributor"
  principal_id         = azuread_service_principal.cluster.id
}
