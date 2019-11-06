resource "digitalocean_database_cluster" "cache" {
  name       = "${var.name}-router"
  engine     = "redis"
  size       = "db-s-1vcpu-1gb"
  region     = var.region
  node_count = 1
}
