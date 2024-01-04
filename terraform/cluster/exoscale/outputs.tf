output "endpoint" {
  value = exoscale_sks_cluster.rack.endpoint
}

output "id" {
  value = exoscale_sks_cluster.rack.id
}

output "depend_id" {
  value = "${exoscale_sks_cluster.rack.id}_${exoscale_sks_nodepool.sks_nodepool.id}" // to create dependency
}

output "name" {
  value = exoscale_sks_cluster.rack.name
}

output "kubeconfigraw" {
  value = exoscale_sks_kubeconfig.sks_kubeconfig.kubeconfig
}
