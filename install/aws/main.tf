variable "name" {
  description = "rack name"
  default     = "convox"
}

variable "node_type" {
  description = "machine type of the cluster nodes"
  default     = "t3.small"
}

variable "release" {
  description = "convox release version to install"
  default     = ""
}

variable "region" {
  description = "aws region in which to install the rack"
  default     = "us-east-1"
}

provider "aws" {
  version = "~> 2.22"

  region = var.region
}

module "system" {
  source = "../../terraform/system/aws"

  name      = var.name
  node_type = var.node_type
  release   = var.release

  providers = {
    aws = aws
  }
}

output "rack_url" {
  value = module.system.api
}
