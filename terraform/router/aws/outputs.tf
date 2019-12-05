output "endpoint" {
  value = data.http.alias.body
}

output "resolver" {
  value = module.k8s.resolver
}
