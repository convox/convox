output "api" {
  value = module.rack.api
}

output "cluster" {
  value = module.cluster
}

output "endpoint" {
  value = module.rack.endpoint
}

output "release" {
  value = local.release
}
