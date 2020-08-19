resource "azurerm_storage_account" "storage" {
  name                     = "${local.prefix}${random_string.suffix.result}"
  resource_group_name      = var.name
  location                 = var.region
  account_tier             = "Standard"
  account_replication_type = "LRS"

  tags = {
    resource_group_id = var.resource_group
  }
}

resource "azurerm_storage_share" "storage" {
  name                 = "storage"
  storage_account_name = azurerm_storage_account.storage.name
}
