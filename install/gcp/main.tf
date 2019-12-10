variable "name" {
  description = "rack name"
  default     = "convox"
}

variable "node_type" {
  description = "machine type of the cluster nodes"
  default     = "n1-standard-1"
}

variable "release" {
  description = "convox release version to install"
  default     = ""
}

module "system" {
  source = "../../terraform/system/gcp"

  name      = var.name
  node_type = var.node_type
  release   = var.release
}

output "rack_url" {
  value = module.system.api
}
