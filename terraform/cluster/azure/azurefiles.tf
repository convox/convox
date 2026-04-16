// Azure Files NFS storage class and storage account
// Gated by azure_files_enable variable
// AKS includes the azurefile-csi driver by default, so no CSI driver installation is needed
//
// NFS requires:
// 1. https_traffic_only_enabled = false (NFS does not use TLS, AKS CSI uses standard mount)
// 2. Private endpoint for VNet-level network access (Azure denies NFS without it)
// 3. Private DNS zone so pods resolve the storage account to the private endpoint IP

resource "azurerm_storage_account" "azurefiles" {
  count = var.azure_files_enable ? 1 : 0

  name                       = "af${substr(replace(var.name, "/[^a-z0-9]/", ""), 0, 16)}${random_string.azurefiles_suffix[0].result}"
  resource_group_name        = var.resource_group_name
  location                   = var.resource_group_location
  account_tier               = "Premium"
  account_kind               = "FileStorage"
  account_replication_type   = "LRS"
  https_traffic_only_enabled = false

  network_rules {
    default_action = "Deny"
    bypass         = ["AzureServices"]
  }

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
    protocol       = "nfs"
    skuName        = "Premium_LRS"
    resourceGroup  = var.resource_group_name
    storageAccount = azurerm_storage_account.azurefiles[0].name
  }

  mount_options = [
    "nconnect=4",
  ]
}

// Discover AKS-managed VNet and subnet in the MC_ resource group.
// AKS kubenet creates exactly one VNet with a subnet named "aks-subnet".

data "azurerm_resources" "aks_vnet" {
  count               = var.azure_files_enable ? 1 : 0
  resource_group_name = azurerm_kubernetes_cluster.rack.node_resource_group
  type                = "Microsoft.Network/virtualNetworks"
}

data "azurerm_subnet" "aks_default" {
  count                = var.azure_files_enable ? 1 : 0
  name                 = "aks-subnet"
  virtual_network_name = data.azurerm_resources.aks_vnet[0].resources[0].name
  resource_group_name  = azurerm_kubernetes_cluster.rack.node_resource_group
}

// Private endpoint for Azure Files NFS access from AKS pods.
// private_dns_zone_group auto-manages the DNS A record lifecycle.

resource "azurerm_private_endpoint" "azurefiles" {
  count               = var.azure_files_enable ? 1 : 0
  name                = "${var.name}-azurefiles-pe"
  location            = var.resource_group_location
  resource_group_name = var.resource_group_name
  subnet_id           = data.azurerm_subnet.aks_default[0].id

  private_service_connection {
    name                           = "${var.name}-azurefiles-psc"
    private_connection_resource_id = azurerm_storage_account.azurefiles[0].id
    is_manual_connection           = false
    subresource_names              = ["file"]
  }

  private_dns_zone_group {
    name                 = "azurefiles-dns"
    private_dns_zone_ids = [azurerm_private_dns_zone.azurefiles[0].id]
  }
}

resource "azurerm_private_dns_zone" "azurefiles" {
  count               = var.azure_files_enable ? 1 : 0
  name                = "privatelink.file.core.windows.net"
  resource_group_name = var.resource_group_name
}

resource "azurerm_private_dns_zone_virtual_network_link" "azurefiles" {
  count                 = var.azure_files_enable ? 1 : 0
  name                  = "${var.name}-azurefiles-dnslink"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.azurefiles[0].name
  virtual_network_id    = data.azurerm_resources.aks_vnet[0].resources[0].id
}
