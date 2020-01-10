resource "azurerm_storage_account" "storage" {
  name                     = "${format("%.12s", var.name)}${random_string.suffix.result}"
  resource_group_name      = data.azurerm_resource_group.rack.name
  location                 = data.azurerm_resource_group.rack.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_storage_share" "storage" {
  name                 = "storage"
  storage_account_name = azurerm_storage_account.storage.name
}
