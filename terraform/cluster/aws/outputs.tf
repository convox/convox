output "id" {
  depends_on = [local_file.kubeconfig, kubernetes_config_map.auth]
  value      = aws_eks_cluster.cluster.id
}

output "kubeconfig" {
  depends_on = [local_file.kubeconfig, kubernetes_config_map.auth]
  value      = local_file.kubeconfig.filename
}

output "nodes_security" {
  value = aws_security_group.nodes.id
}

output "oidc_arn" {
  value = aws_iam_openid_connect_provider.cluster.arn
}

output "oidc_sub" {
  value = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
}

output "subnets_private" {
  value = aws_subnet.private.*.id
}

output "subnets_public" {
  value = aws_subnet.public.*.id
}

output "target_group_http" {
  value = aws_cloudformation_stack.nodes.outputs.RouterTargetGroup80
}

output "target_group_https" {
  value = aws_cloudformation_stack.nodes.outputs.RouterTargetGroup443
}
