output "addr" {
  value = "${azurerm_redis_cache.redis.hostname}:${azurerm_redis_cache.redis.ssl_port}"
}

output "auth" {
  value = azurerm_redis_cache.redis.primary_access_key
}

output "host" {
  value = azurerm_redis_cache.redis.hostname
}

output "port" {
  value = azurerm_redis_cache.redis.ssl_port
}

output "url" {
  value = "redisis://${azurerm_redis_cache.redis.primary_access_key}@${azurerm_redis_cache.redis.hostname}:${azurerm_redis_cache.redis.ssl_port}"
}
