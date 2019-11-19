resource "azurerm_redis_cache" "cache" {
  name                = "${var.name}-router"
  location            = data.azurerm_resource_group.rack.location
  resource_group_name = data.azurerm_resource_group.rack.name
  capacity            = 0
  family              = "C"
  sku_name            = "Basic"
}
