# ALL depends_on are required in order of kubernetes provider to work
# DO NOT remove depends_on tag
output "ca" {
  depends_on = [null_resource.wait_k8s_cluster]
  value      = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
}

output "ebs_csi_driver_name" {
  value = module.ebs_csi_driver_controller.ebs_csi_driver_name
}

output "endpoint" {
  depends_on = [null_resource.wait_k8s_cluster]
  value      = aws_eks_cluster.cluster.endpoint
}

output "id" {
  depends_on = [null_resource.wait_k8s_cluster]
  value      = aws_eks_cluster.cluster.id
}

output "oidc_arn" {
  depends_on = [null_resource.wait_k8s_cluster]
  value      = aws_iam_openid_connect_provider.cluster.arn
}

output "oidc_sub" {
  depends_on = [null_resource.wait_k8s_cluster]
  value      = local.oidc_sub
}

output "route_table_public" {
  value = aws_route_table.public.id
}

output "subnets" {
  value = aws_subnet.private.*.id
}

output "vpc" {
  value = var.vpc_id == "" ? aws_vpc.nodes[0].id : var.vpc_id
}
