resource "azurerm_container_registry" "registry" {
  name                = "${local.prefix}${random_string.suffix.result}"
  resource_group_name = var.name
  location            = var.region
  sku                 = "Basic"
}
