variable "name" {
  description = "rack name"
  default     = "convox"
}

variable "node_type" {
  description = "machine type of the cluster nodes"
  default     = "Standard_D1_v2"
}

variable "release" {
  description = "convox release version to install"
  default     = ""
}

variable "region" {
  description = "region in which to install the rack"
  default     = "eastus"
}

provider "azurerm" {
  version = "~> 1.37"
}

module "system" {
  source = "../../terraform/system/azure"

  providers = {
    azurerm = azurerm
  }

  name      = var.name
  node_type = var.node_type
  release   = var.release
  region    = var.region
}

output "rack_url" {
  value = module.system.api
}
