output "addr" {
  value = "${local.host}:${local.port}"
}

output "host" {
  value = local.host
}

output "port" {
  value = local.port
}

output "url" {
  value = "redis://${local.host}:${local.port}"
}
