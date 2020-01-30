provider "aws" {
  version = "~> 2.22"
}

resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.name}-${random_string.suffix.result}-subnets"
  subnet_ids = var.subnets
}

resource "aws_elasticache_replication_group" "redis" {
  automatic_failover_enabled    = true
  engine                        = "redis"
  engine_version                = "4.0.10"
  node_type                     = "cache.t2.micro"
  number_cache_clusters         = 2
  parameter_group_name          = "default.redis4.0"
  port                          = 6379
  replication_group_id          = "${var.name}-${random_string.suffix.result}"
  replication_group_description = var.name
  subnet_group_name             = aws_elasticache_subnet_group.redis.name
}
