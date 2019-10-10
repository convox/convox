resource "google_redis_instance" "cache" {
  name           = "${var.name}-router"
  memory_size_gb = 1
}
