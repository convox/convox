output "addr" {
  value = "${digitalocean_database_cluster.redis.private_host}:${digitalocean_database_cluster.redis.port}"
}

output "auth" {
  value = digitalocean_database_cluster.redis.password
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

