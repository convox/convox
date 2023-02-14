output "api" {
  value = module.rack.api
}

output "cluster" {
  value = module.cluster
}

output "endpoint" {
  value = module.rack.endpoint
}

output "endpoint_internal" {
  value = module.rack.endpoint_internal
}

output "release" {
  value = local.release
}
