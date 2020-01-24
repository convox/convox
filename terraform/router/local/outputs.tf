output "endpoint" {
  value = "${var.name}.convox"
}

output "resolver" {
  value = module.k8s.resolver
}

