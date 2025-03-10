resource "random_id" "additional_node_groups" {
  byte_length = 8

  count = length(var.additional_node_groups)

  keepers = {
    node_capacity_type  = var.additional_node_groups[count.index].capacity_type != null ? var.additional_node_groups[count.index].capacity_type : "ON_DEMAND"
    node_disk           = var.additional_node_groups[count.index].disk != null ? var.additional_node_groups[count.index].disk : var.node_disk
    node_type           = var.additional_node_groups[count.index].type
    ami_id              = var.additional_node_groups[count.index].ami_id
    private             = var.private
    private_subnets_ids = join("-", local.private_subnets_ids)
    public_subnets_ids  = join("-", local.public_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

module "amitype" {
  source    = "../../helpers/aws"
  for_each  = { for idx, ng in var.additional_node_groups : idx => ng }
  node_type = each.value.type
}

resource "aws_eks_node_group" "cluster_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = length(var.additional_node_groups)

  ami_type        = random_id.additional_node_groups[count.index].keepers.ami_id != null ? "CUSTOM" : module.amitype[count.index].ami_type
  capacity_type   = random_id.additional_node_groups[count.index].keepers.node_capacity_type
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-additional-${count.index}-${random_id.additional_node_groups[count.index].hex}"
  node_role_arn   = random_id.additional_node_groups[count.index].keepers.role_arn
  subnet_ids      = var.private ? local.private_subnets_ids : local.public_subnets_ids
  tags            = local.tags
  version         = var.k8s_version

  launch_template {
    id      = aws_launch_template.cluster_additional[count.index].id
    version = "$Latest"
  }

  scaling_config {
    desired_size = var.additional_node_groups[count.index].desired_size != null ? var.additional_node_groups[count.index].desired_size : 1
    min_size     = var.additional_node_groups[count.index].min_size != null ? var.additional_node_groups[count.index].min_size : 1
    max_size     = var.additional_node_groups[count.index].max_size != null ? var.additional_node_groups[count.index].max_size : 100
  }

  dynamic "update_config" {
    for_each = var.node_max_unavailable_percentage > 0 ? [var.node_max_unavailable_percentage] : []
    content {
      max_unavailable_percentage = var.node_max_unavailable_percentage
    }
  }

  lifecycle {
    create_before_destroy = true
    ignore_changes        = [scaling_config[0].desired_size]
  }

  labels = {
    "convox.io/label" = var.additional_node_groups[count.index].label != null ? var.additional_node_groups[count.index].label : "custom"
  }

  dynamic "taint" {
    for_each = var.additional_node_groups[count.index].dedicated ? [1] : []
    content {
      key    = "dedicated-node"
      value  = var.additional_node_groups[count.index].label != null ? var.additional_node_groups[count.index].label : "custom"
      effect = "NO_SCHEDULE"
    }
  }

  timeouts {
    update = "2h"
    delete = "1h"
    create = "1h"
  }
}

resource "aws_launch_template" "cluster_additional" {
  count = length(var.additional_node_groups)

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = random_id.additional_node_groups[count.index].keepers.node_disk
    }
  }

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  instance_type = random_id.additional_node_groups[count.index].keepers.node_type

  image_id = random_id.additional_node_groups[count.index].keepers.ami_id

  dynamic "tag_specifications" {
    for_each = toset(
      concat(["instance", "volume", "network-interface", "spot-instances-request"],
        var.gpu_tag_enable ? ["elastic-gpu"] : []
    ))
    content {
      resource_type = tag_specifications.key
      tags          = local.tags
    }
  }

  user_data = var.user_data_url != "" || var.user_data != "" || var.kubelet_registry_pull_qps != 5 || var.kubelet_registry_burst != 10 ? base64encode(templatefile("${path.module}/files/custom_user_data.sh", {
    kubelet_registry_pull_qps = var.kubelet_registry_pull_qps
    kubelet_registry_burst    = var.kubelet_registry_burst
    user_data_script_file     = var.user_data_url != "" ? data.http.user_data_content[0].response_body : ""
    user_data                 = var.user_data
  })) : ""

  key_name = var.key_pair_name != "" ? var.key_pair_name : null
}

resource "aws_autoscaling_group_tag" "cluster_additional" {
  depends_on = [
    aws_eks_node_group.cluster_additional
  ]

  count = length(var.additional_node_groups)

  autoscaling_group_name = aws_eks_node_group.cluster_additional[count.index].resources[0].autoscaling_groups[0].name
  tag {
    key   = "k8s.io/cluster-autoscaler/node-template/label/convox.io/label"
    value =  var.additional_node_groups[count.index].label != null ? var.additional_node_groups[count.index].label : "custom"

    propagate_at_launch = true
  }
}

###### additional build node groups

resource "random_id" "build_node_additional" {
  count = length(var.additional_build_groups)

  byte_length = 8

  keepers = {
    node_disk           = var.additional_build_groups[count.index].disk != null ? var.additional_build_groups[count.index].disk : var.node_disk
    node_type           = var.additional_build_groups[count.index].type
    ami_id              = var.additional_build_groups[count.index].ami_id
    private_subnets_ids = join("-", local.private_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

module "build_amitype" {
  source    = "../../helpers/aws"
  for_each  = { for idx, ng in var.additional_build_groups : idx => ng }
  node_type = each.value.type
}


resource "aws_eks_node_group" "build_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = length(var.additional_build_groups)

  ami_type        = random_id.build_node_additional[count.index].keepers.ami_id != null ? "CUSTOM" : module.build_amitype[count.index].ami_type
  capacity_type   = var.additional_build_groups[count.index].capacity_type != null ? var.additional_build_groups[count.index].capacity_type : "ON_DEMAND"
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-build-additional-${count.index}-${random_id.build_node_additional[count.index].hex}"
  node_role_arn   = random_id.build_node_additional[count.index].keepers.role_arn
  subnet_ids      = local.private_subnets_ids
  tags            = local.tags
  version         = var.k8s_version

  labels = {
    "convox-build" : "true"
    "convox.io/label" = var.additional_build_groups[count.index].label != null ? var.additional_build_groups[count.index].label : "custom-build"
  }

  taint {
    key    = "dedicated"
    value  = "build"
    effect = "NO_SCHEDULE"
  }

  launch_template {
    id      = aws_launch_template.build_additional[count.index].id
    version = "$Latest"
  }

  scaling_config {
    desired_size = var.additional_build_groups[count.index].desired_size != null ? var.additional_build_groups[count.index].desired_size : 0
    min_size     = var.additional_build_groups[count.index].min_size != null ? var.additional_build_groups[count.index].min_size : 0
    max_size     = var.additional_build_groups[count.index].max_size != null ? var.additional_build_groups[count.index].max_size : 100
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_launch_template" "build_additional" {
  count = length(var.additional_build_groups)

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = random_id.build_node_additional[count.index].keepers.node_disk
    }
  }

  instance_type = random_id.build_node_additional[count.index].keepers.node_type

  image_id = random_id.build_node_additional[count.index].keepers.ami_id

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  dynamic "tag_specifications" {
    for_each = toset(
      concat(["instance", "volume", "network-interface", "spot-instances-request"],
        var.gpu_tag_enable ? ["elastic-gpu"] : []
    ))
    content {
      resource_type = tag_specifications.key
      tags          = local.tags
    }
  }

  key_name = var.key_pair_name != "" ? var.key_pair_name : null
}

resource "aws_autoscaling_group_tag" "build_additional" {
  depends_on = [
    aws_eks_node_group.build_additional
  ]

  count = var.build_node_enabled ? length(var.additional_build_groups) : 0

  autoscaling_group_name = aws_eks_node_group.build_additional[count.index].resources[0].autoscaling_groups[0].name

  tag {
    key   = "k8s.io/cluster-autoscaler/node-template/label/convox-build"
    value = "true"

    propagate_at_launch = true
  }
}

resource "aws_autoscaling_group_tag" "build_additional_custom" {
  depends_on = [
    aws_eks_node_group.build_additional
  ]

  count = length(var.additional_build_groups)

  autoscaling_group_name = aws_eks_node_group.build_additional[count.index].resources[0].autoscaling_groups[0].name
  tag {
    key   = "k8s.io/cluster-autoscaler/node-template/label/convox.io/label"
    value =  var.additional_build_groups[count.index].label != null ? var.additional_build_groups[count.index].label : "custom-build"

    propagate_at_launch = true
  }
}
