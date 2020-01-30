provider "azurerm" {
  version = "~> 1.37"
}

data "azurerm_resource_group" "rack" {
  name = var.resource_group
}

resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "azurerm_redis_cache" "redis" {
  name = "${var.name}-${random_string.suffix.result}"

  capacity            = 0
  family              = "C"
  location            = data.azurerm_resource_group.rack.location
  resource_group_name = data.azurerm_resource_group.rack.name
  sku_name            = "Standard"
}
