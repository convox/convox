provider "google" {
  version = "~> 3.5.0"
}

resource "google_redis_instance" "redis" {
  name = "${var.name}-router"

  authorized_network = var.network
  memory_size_gb     = 1
  tier               = "STANDARD_HA"
}
