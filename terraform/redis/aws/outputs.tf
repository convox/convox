output "addr" {
  value = "${aws_elasticache_replication_group.redis.primary_endpoint_address}:${aws_elasticache_replication_group.redis.port}"
}

output "host" {
  value = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "port" {
  value = aws_elasticache_replication_group.redis.port
}

output "url" {
  value = "redis://${aws_elasticache_replication_group.redis.primary_endpoint_address}:${aws_elasticache_replication_group.redis.port}"
}

