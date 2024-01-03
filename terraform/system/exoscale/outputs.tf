output "endpoint" {
  value = module.cluster.endpoint
}

output "test" {
  value = local.kube_config_yaml
}