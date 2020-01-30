output "ca" {
  depends_on = [aws_eks_node_group.cluster]
  value      = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
}

output "endpoint" {
  depends_on = [aws_eks_node_group.cluster]
  value      = aws_eks_cluster.cluster.endpoint
}

output "id" {
  depends_on = [aws_eks_node_group.cluster]
  value      = aws_eks_cluster.cluster.id
}

output "oidc_arn" {
  depends_on = [aws_eks_node_group.cluster]
  value      = aws_iam_openid_connect_provider.cluster.arn
}

output "oidc_sub" {
  depends_on = [aws_eks_node_group.cluster]
  value      = local.oidc_sub
}

output "subnets" {
  value = aws_subnet.private.*.id
}
