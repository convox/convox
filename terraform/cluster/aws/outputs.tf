output "ca" {
  depends_on = [kubernetes_config_map.auth]
  value      = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
}

output "endpoint" {
  depends_on = [kubernetes_config_map.auth]
  value      = aws_eks_cluster.cluster.endpoint
}

output "id" {
  depends_on = [kubernetes_config_map.auth]
  value      = aws_eks_cluster.cluster.id
}

output "oidc_arn" {
  value = aws_iam_openid_connect_provider.cluster.arn
}

output "oidc_sub" {
  value = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
}
