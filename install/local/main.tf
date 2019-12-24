variable "name" {
  description = "rack name"
  default     = "convox"
}

variable "release" {
  description = "convox release version to install"
  default     = ""
}

module "system" {
  source = "../../terraform/system/local"

  name    = var.name
  release = var.release
}

output "rack_url" {
  value = module.system.api
}
