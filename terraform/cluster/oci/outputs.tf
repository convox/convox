output "ca" {
  depends_on = [oci_containerengine_node_pool.rack]
  value      = local.kube_ca
}

output "endpoint" {
  depends_on = [oci_containerengine_node_pool.rack]
  value      = local.kube_host
}

output "id" {
  depends_on = [oci_containerengine_node_pool.rack]
  value      = oci_containerengine_cluster.rack.id
}

output "name" {
  value = oci_containerengine_cluster.rack.name
}
