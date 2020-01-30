output "addr" {
  value = "${digitalocean_database_cluster.redis.private_host}:${digitalocean_database_cluster.redis.port}"
}

output "host" {
  value = digitalocean_database_cluster.redis.private_host
}

output "port" {
  value = digitalocean_database_cluster.redis.port
}

output "url" {
  value = digitalocean_database_cluster.redis.private_uri
}

