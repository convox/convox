variable "access_id" {
  description = "spaces access id"
  type        = "string"
}

variable "name" {
  description = "rack name"
  default     = "convox"
}

variable "node_type" {
  description = "machine type of the cluster nodes"
  default     = "s-1vcpu-2gb"
}

variable "release" {
  description = "convox release version to install"
  default     = ""
}

variable "region" {
  description = "region in which to install the rack"
  default     = "nyc3"
}

variable "registry_disk" {
  description = "size of the registry disk volume"
  default     = "50Gi"
}

variable "secret_key" {
  description = "spaces secret key"
  type        = "string"
}

variable "token" {
  description = "digital ocean api token"
  type        = "string"
}

provider "digitalocean" {
  version = "~> 1.9"

  token = var.token

  spaces_access_id  = var.access_id
  spaces_secret_key = var.secret_key
}

module "system" {
  source = "../../terraform/system/do"

  access_id     = var.access_id
  name          = var.name
  node_type     = var.node_type
  release       = var.release
  region        = var.region
  registry_disk = var.registry_disk
  secret_key    = var.secret_key
  token         = var.token

  providers = {
    digitalocean = digitalocean
  }
}

output "rack_url" {
  value = module.system.api
}
