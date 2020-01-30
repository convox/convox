provider "digitalocean" {
  version = "~> 1.11"
}

resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "digitalocean_database_cluster" "redis" {
  name       = "${var.name}-${random_string.suffix.result}"
  engine     = "redis"
  node_count = 1
  size       = "db-s-1vcpu-1gb"
  region     = var.region

  lifecycle {
    ignore_changes = [version]
  }
}
