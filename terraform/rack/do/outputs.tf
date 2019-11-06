output "api" {
  value = module.api.endpoint
}

output "elasticsearch" {
  value = module.elasticsearch.host
}

output "endpoint" {
  value = module.router.endpoint
}
