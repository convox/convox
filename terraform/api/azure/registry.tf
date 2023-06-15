resource "azurerm_container_registry" "registry" {
  name                = "${local.prefix}${random_string.suffix.result}"
  resource_group_name = var.resource_group_name
  location            = var.resource_group_location
  sku                 = "Basic"
}
