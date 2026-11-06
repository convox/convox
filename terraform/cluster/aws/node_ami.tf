locals {
  # managed node groups on an EKS AMI release older than this move once to the newest release
  node_ami_release_floor = 20261031

  node_ami_ssm_path = {
    AL2023_x86_64_STANDARD = "amazon-linux-2023/x86_64/standard"
    AL2023_ARM_64_STANDARD = "amazon-linux-2023/arm64/standard"
    AL2023_x86_64_NVIDIA   = "amazon-linux-2023/x86_64/nvidia"
  }

  node_ami_stale = {
    for name, ng in data.aws_eks_node_group.current : name => ng.ami_type
    if contains(keys(local.node_ami_ssm_path), ng.ami_type) && try(tonumber(element(split("-", ng.release_version), 1)), local.node_ami_release_floor) < local.node_ami_release_floor
  }

  node_ami_release = {
    for name, ami_type in local.node_ami_stale : name => data.aws_ssm_parameter.node_ami_release[ami_type].insecure_value
    if try(tonumber(element(split("-", data.aws_ssm_parameter.node_ami_release[ami_type].insecure_value), 1)), 0) >= local.node_ami_release_floor
  }
}

data "aws_eks_clusters" "all" {}

data "aws_eks_node_groups" "current" {
  count = contains(data.aws_eks_clusters.all.names, var.name) ? 1 : 0

  cluster_name = var.name
}

data "aws_eks_node_group" "current" {
  for_each = toset(flatten([for ngs in data.aws_eks_node_groups.current : tolist(ngs.names)]))

  cluster_name    = var.name
  node_group_name = each.value
}

data "aws_ssm_parameter" "node_ami_release" {
  for_each = toset(values(local.node_ami_stale))

  name = "/aws/service/eks/optimized-ami/${var.k8s_version}/${local.node_ami_ssm_path[each.value]}/recommended/release_version"
}
