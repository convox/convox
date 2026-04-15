// Azure Files NFS storage class and storage account
// Gated by azure_files_enable variable
// AKS includes the azurefile-csi driver by default, so no CSI driver installation is needed

resource "azurerm_storage_account" "azurefiles" {
  count = var.azure_files_enable ? 1 : 0

  name                     = "af${substr(replace(var.name, "/[^a-z0-9]/", ""), 0, 16)}${random_string.azurefiles_suffix[0].result}"
  resource_group_name      = var.resource_group_name
  location                 = var.resource_group_location
  account_tier             = "Premium"
  account_kind             = "FileStorage"
  account_replication_type = "LRS"

  tags = {
    system = "convox"
    rack   = var.name
  }
}

resource "random_string" "azurefiles_suffix" {
  count = var.azure_files_enable ? 1 : 0

  length  = 6
  special = false
  upper   = false
}

resource "kubernetes_storage_class_v1" "azurefile_csi_nfs" {
  count = var.azure_files_enable ? 1 : 0

  metadata {
    name = "azurefile-csi-nfs"
  }

  storage_provisioner    = "file.csi.azure.com"
  reclaim_policy         = "Delete"
  volume_binding_mode    = "Immediate"
  allow_volume_expansion = true

  parameters = {
    protocol         = "nfs"
    skuName          = "Premium_LRS"
    resourceGroup    = var.resource_group_name
    storageAccount   = azurerm_storage_account.azurefiles[0].name
  }

  mount_options = [
    "nconnect=4",
  ]
}
