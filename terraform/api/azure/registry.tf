resource "azurerm_container_registry" "registry" {
  name                = "${format("%.12s", var.name)}${random_string.suffix.result}"
  resource_group_name = "${data.azurerm_resource_group.rack.name}"
  location            = "${data.azurerm_resource_group.rack.location}"
  sku                 = "Basic"
}
