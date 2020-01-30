output "addr" {
  value = "${google_redis_instance.redis.host}:${google_redis_instance.redis.port}"
}

output "host" {
  value = google_redis_instance.redis.host
}

output "port" {
  value = google_redis_instance.redis.port
}

output "url" {
  value = "redis://${google_redis_instance.redis.host}:${google_redis_instance.redis.port}"
}
