variable "credentials" {
  description = "path to credentials, create at https://console.cloud.google.com/apis/credentials/serviceaccountkey"
  default     = "~/.config/gcloud/terraform.json"
}

variable "name" {
  description = "rack name"
  default     = "convox"
}

variable "node_type" {
  description = "machine type of the cluster nodes"
  default     = "n1-standard-1"
}

variable "project" {
  description = "id of gcp project in which to install the rack"
  type        = string
}

variable "release" {
  description = "convox release version to install"
  default     = ""
}

variable "region" {
  description = "gcp region in which to install the rack"
  default     = "us-east1"
}

provider "google" {
  version = "~> 2.19"

  credentials = pathexpand(var.credentials)
  project     = var.project
  region      = var.region
}

provider "google-beta" {
  version = "~> 2.19"

  credentials = pathexpand(var.credentials)
  project     = var.project
  region      = var.region
}

module "system" {
  source = "../../terraform/system/gcp"

  name      = var.name
  node_type = var.node_type
  release   = var.release

  providers = {
    google      = google
    google-beta = google-beta
  }
}

output "rack_url" {
  value = module.system.api
}
