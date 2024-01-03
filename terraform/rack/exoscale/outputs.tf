output "api" {
  value = module.api.endpoint
}

output "endpoint" {
  value = module.router.endpoint
}

output "endpoint_internal" {
  value = module.router.endpoint_internal
}
