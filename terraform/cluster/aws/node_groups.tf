
locals {
  launch_template_user_data_raw = var.user_data_url != "" || var.user_data != "" || var.kubelet_registry_pull_qps != 5 || var.kubelet_registry_burst != 10 ? templatefile("${path.module}/files/custom_user_data.sh", {
    kubelet_registry_pull_qps = var.kubelet_registry_pull_qps
    kubelet_registry_burst    = var.kubelet_registry_burst
    user_data_script_file     = var.user_data_url != "" ? data.http.user_data_content[0].response_body : ""
    user_data                 = var.user_data
  }) : ""

  kube_dns_ip = cidrhost(aws_eks_cluster.cluster.kubernetes_network_config[0].service_ipv4_cidr, 10)

  additional_node_groups_with_defaults = [
    for ng in var.additional_node_groups : {
      type          = lookup(ng, "type", null)
      disk          = tonumber(lookup(ng, "disk", var.node_disk))
      capacity_type = lookup(ng, "capacity_type", "ON_DEMAND")
      min_size      = tonumber(lookup(ng, "min_size", 1))
      desired_size  = tonumber(lookup(ng, "min_size", 1))
      max_size      = tonumber(lookup(ng, "max_size", 100))
      label         = lookup(ng, "label", null)
      ami_id        = lookup(ng, "ami_id", null)
      dedicated     = tobool(lookup(ng, "dedicated", false))
    }
  ]

  additional_build_groups_with_defaults = [
    for ng in var.additional_build_groups : {
      type          = lookup(ng, "type", null)
      disk          = tonumber(lookup(ng, "disk", var.node_disk))
      capacity_type = lookup(ng, "capacity_type", "ON_DEMAND")
      min_size      = tonumber(lookup(ng, "min_size", 0))
      desired_size  = tonumber(lookup(ng, "min_size", 0))
      max_size      = tonumber(lookup(ng, "max_size", 100))
      label         = lookup(ng, "label", null)
      ami_id        = lookup(ng, "ami_id", null)
    }
  ]
}

resource "random_id" "additional_node_groups" {
  byte_length = 8

  count = length(local.additional_node_groups_with_defaults)

  keepers = {
    node_capacity_type  = local.additional_node_groups_with_defaults[count.index].capacity_type != null ? local.additional_node_groups_with_defaults[count.index].capacity_type : "ON_DEMAND"
    node_disk           = local.additional_node_groups_with_defaults[count.index].disk != null ? local.additional_node_groups_with_defaults[count.index].disk : var.node_disk
    node_type           = local.additional_node_groups_with_defaults[count.index].type
    ami_id              = local.additional_node_groups_with_defaults[count.index].ami_id
    private             = var.private
    private_subnets_ids = join("-", local.private_subnets_ids)
    public_subnets_ids  = join("-", local.public_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

module "amitype" {
  source    = "../../helpers/aws"
  for_each  = { for idx, ng in local.additional_node_groups_with_defaults : idx => ng }
  node_type = each.value.type
}

resource "aws_eks_node_group" "cluster_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = length(local.additional_node_groups_with_defaults)

  ami_type        = random_id.additional_node_groups[count.index].keepers.ami_id != null ? "CUSTOM" : module.amitype[count.index].ami_type
  capacity_type   = random_id.additional_node_groups[count.index].keepers.node_capacity_type
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-additional-${count.index}-${random_id.additional_node_groups[count.index].hex}"
  node_role_arn   = random_id.additional_node_groups[count.index].keepers.role_arn
  subnet_ids      = var.private ? local.private_subnets_ids : local.public_subnets_ids
  tags            = local.tags
  version         = random_id.additional_node_groups[count.index].keepers.ami_id != null ? null : var.k8s_version

  launch_template {
    id      = aws_launch_template.cluster_additional[count.index].id
    version = "$Latest"
  }

  scaling_config {
    desired_size = local.additional_node_groups_with_defaults[count.index].desired_size != null ? local.additional_node_groups_with_defaults[count.index].desired_size : 1
    min_size     = local.additional_node_groups_with_defaults[count.index].min_size != null ? local.additional_node_groups_with_defaults[count.index].min_size : 1
    max_size     = local.additional_node_groups_with_defaults[count.index].max_size != null ? local.additional_node_groups_with_defaults[count.index].max_size : 100
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
    "convox.io/label" = local.additional_node_groups_with_defaults[count.index].label != null ? local.additional_node_groups_with_defaults[count.index].label : "custom"
  }

  dynamic "taint" {
    for_each = local.additional_node_groups_with_defaults[count.index].dedicated ? [1] : []
    content {
      key    = "dedicated-node"
      value  = local.additional_node_groups_with_defaults[count.index].label != null ? local.additional_node_groups_with_defaults[count.index].label : "custom"
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
  count = length(local.additional_node_groups_with_defaults)

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

  user_data = random_id.additional_node_groups[count.index].keepers.ami_id == null ? null : base64encode(<<-EOF
#!/bin/bash
set -ex
/etc/eks/bootstrap.sh ${aws_eks_cluster.cluster.name} \
  --kubelet-extra-args '--node-labels=eks.amazonaws.com/nodegroup=${var.name}-additional-${count.index}-${random_id.additional_node_groups[count.index].hex}' \
  --b64-cluster-ca ${aws_eks_cluster.cluster.certificate_authority[0].data} \
  --apiserver-endpoint ${aws_eks_cluster.cluster.endpoint} --use-max-pods true --dns-cluster-ip ${local.kube_dns_ip}

${local.launch_template_user_data_raw}
EOF
  )
  key_name = var.key_pair_name != "" ? var.key_pair_name : null
}

resource "aws_autoscaling_group_tag" "cluster_additional" {
  depends_on = [
    aws_eks_node_group.cluster_additional
  ]

  count = length(local.additional_node_groups_with_defaults)

  autoscaling_group_name = aws_eks_node_group.cluster_additional[count.index].resources[0].autoscaling_groups[0].name
  tag {
    key   = "k8s.io/cluster-autoscaler/node-template/label/convox.io/label"
    value = local.additional_node_groups_with_defaults[count.index].label != null ? local.additional_node_groups_with_defaults[count.index].label : "custom"

    propagate_at_launch = true
  }
}

###### additional build node groups

resource "random_id" "build_node_additional" {
  count = length(local.additional_build_groups_with_defaults)

  byte_length = 8

  keepers = {
    node_disk           = local.additional_build_groups_with_defaults[count.index].disk != null ? local.additional_build_groups_with_defaults[count.index].disk : var.node_disk
    node_type           = local.additional_build_groups_with_defaults[count.index].type
    capacity_type       = local.additional_build_groups_with_defaults[count.index].capacity_type
    ami_id              = local.additional_build_groups_with_defaults[count.index].ami_id
    private_subnets_ids = join("-", local.private_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

module "build_amitype" {
  source    = "../../helpers/aws"
  for_each  = { for idx, ng in local.additional_build_groups_with_defaults : idx => ng }
  node_type = each.value.type
}


resource "aws_eks_node_group" "build_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = length(local.additional_build_groups_with_defaults)

  ami_type        = random_id.build_node_additional[count.index].keepers.ami_id != null ? "CUSTOM" : module.build_amitype[count.index].ami_type
  capacity_type   = random_id.build_node_additional[count.index].keepers.capacity_type != null ? random_id.build_node_additional[count.index].keepers.capacity_type : "ON_DEMAND"
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-build-additional-${count.index}-${random_id.build_node_additional[count.index].hex}"
  node_role_arn   = random_id.build_node_additional[count.index].keepers.role_arn
  subnet_ids      = local.private_subnets_ids
  tags            = local.tags
  version         = random_id.build_node_additional[count.index].keepers.ami_id != null ? null : var.k8s_version

  labels = {
    "convox-build" : "true"
    "convox.io/label" = local.additional_build_groups_with_defaults[count.index].label != null ? local.additional_build_groups_with_defaults[count.index].label : "custom-build"
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
    desired_size = local.additional_build_groups_with_defaults[count.index].desired_size != null ? local.additional_build_groups_with_defaults[count.index].desired_size : 0
    min_size     = local.additional_build_groups_with_defaults[count.index].min_size != null ? local.additional_build_groups_with_defaults[count.index].min_size : 0
    max_size     = local.additional_build_groups_with_defaults[count.index].max_size != null ? local.additional_build_groups_with_defaults[count.index].max_size : 100
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_launch_template" "build_additional" {
  count = length(local.additional_build_groups_with_defaults)

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

  user_data = random_id.build_node_additional[count.index].keepers.ami_id == null ? null : base64encode(<<-EOF
#!/bin/bash
set -ex
/etc/eks/bootstrap.sh ${aws_eks_cluster.cluster.name} \
  --kubelet-extra-args '--node-labels=eks.amazonaws.com/nodegroup=${var.name}-build-additional-${count.index}-${random_id.build_node_additional[count.index].hex}' \
  --b64-cluster-ca ${aws_eks_cluster.cluster.certificate_authority[0].data} \
  --apiserver-endpoint ${aws_eks_cluster.cluster.endpoint} --use-max-pods true --dns-cluster-ip ${local.kube_dns_ip}
EOF
  )

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

  count = var.build_node_enabled ? length(local.additional_build_groups_with_defaults) : 0

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

  count = length(local.additional_build_groups_with_defaults)

  autoscaling_group_name = aws_eks_node_group.build_additional[count.index].resources[0].autoscaling_groups[0].name
  tag {
    key   = "k8s.io/cluster-autoscaler/node-template/label/convox.io/label"
    value = local.additional_build_groups_with_defaults[count.index].label != null ? local.additional_build_groups_with_defaults[count.index].label : "custom-build"

    propagate_at_launch = true
  }
}
