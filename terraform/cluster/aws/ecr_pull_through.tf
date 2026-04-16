data "aws_caller_identity" "current" {}

resource "aws_secretsmanager_secret" "docker_hub_pull_through" {
  count = var.ecr_docker_hub_cache && var.docker_hub_username != "" && var.docker_hub_password != "" ? 1 : 0

  name                    = "${var.name}-docker-hub-pull-through"
  description             = "Docker Hub credentials for ECR pull-through cache"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "docker_hub_pull_through" {
  count = var.ecr_docker_hub_cache && var.docker_hub_username != "" && var.docker_hub_password != "" ? 1 : 0

  secret_id = aws_secretsmanager_secret.docker_hub_pull_through[0].id
  secret_string = jsonencode({
    username    = var.docker_hub_username
    accessToken = var.docker_hub_password
  })
}

resource "aws_ecr_pull_through_cache_rule" "docker_hub" {
  count = var.ecr_docker_hub_cache && var.docker_hub_username != "" && var.docker_hub_password != "" ? 1 : 0

  ecr_repository_prefix = "docker-hub-${var.name}"
  upstream_registry_url = "registry-1.docker.io"
  credential_arn        = aws_secretsmanager_secret.docker_hub_pull_through[0].arn
}

resource "aws_iam_policy" "ecr_pull_through" {
  count = var.ecr_docker_hub_cache && var.docker_hub_username != "" && var.docker_hub_password != "" ? 1 : 0

  name = "${var.name}-ecr-pull-through"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchImportUpstreamImage",
          "ecr:CreateRepository",
        ]
        Resource = "arn:${data.aws_partition.current.partition}:ecr:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:repository/docker-hub-${var.name}/*"
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "nodes_ecr_pull_through" {
  count = var.ecr_docker_hub_cache && var.docker_hub_username != "" && var.docker_hub_password != "" ? 1 : 0

  role       = aws_iam_role.nodes.name
  policy_arn = aws_iam_policy.ecr_pull_through[0].arn
}

resource "aws_iam_role_policy_attachment" "karpenter_nodes_ecr_pull_through" {
  count = var.ecr_docker_hub_cache && var.docker_hub_username != "" && var.docker_hub_password != "" && var.karpenter_enabled ? 1 : 0

  role       = aws_iam_role.karpenter_nodes[0].name
  policy_arn = aws_iam_policy.ecr_pull_through[0].arn
}
