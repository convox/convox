resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

resource "digitalocean_database_cluster" "cache" {
  name       = "${var.name}-router-${random_string.suffix.result}"
  engine     = "redis"
  size       = "db-s-1vcpu-1gb"
  region     = var.region
  node_count = 1
}
