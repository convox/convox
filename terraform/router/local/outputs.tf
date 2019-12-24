output "endpoint" {
  value = var.name
}

output "resolver" {
  value = module.k8s.resolver
}

