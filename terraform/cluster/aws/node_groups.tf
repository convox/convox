
locals {
  launch_template_user_data_raw = var.user_data_url != "" || var.user_data != "" || var.kubelet_registry_pull_qps != 5 || var.kubelet_registry_burst != 10 ? templatefile("${path.module}/files/custom_user_data.sh", {
    kubelet_registry_pull_qps = var.kubelet_registry_pull_qps
    kubelet_registry_burst    = var.kubelet_registry_burst
    user_data_script_file     = var.user_data_url != "" ? data.http.user_data_content[0].response_body : ""
    user_data                 = var.user_data
  }) : ""

  kube_dns_ip = cidrhost(aws_eks_cluster.cluster.kubernetes_network_config[0].service_ipv4_cidr, 10)

  additional_node_groups_with_defaults = [
    for idx, ng in var.additional_node_groups : {
      id            = tonumber(lookup(ng, "id", idx))
      type          = lookup(ng, "type", null)
      types         = lookup(ng, "types", null)
      cpu           = tonumber(lookup(ng, "cpu", 0))
      mem           = tonumber(lookup(ng, "mem", 0))
      disk          = tonumber(lookup(ng, "disk", var.node_disk))
      capacity_type = lookup(ng, "capacity_type", "ON_DEMAND")
      min_size      = tonumber(lookup(ng, "min_size", 1))
      desired_size  = tonumber(lookup(ng, "min_size", 1))
      max_size      = tonumber(lookup(ng, "max_size", 100))
      label         = lookup(ng, "label", null)
      ami_id        = lookup(ng, "ami_id", null)
      dedicated     = tobool(lookup(ng, "dedicated", false))
      tags = {
        for pair in compact(split(",", lookup(ng, "tags", ""))) :
        trimspace(split("=", pair)[0]) => trimspace(try(split("=", pair)[1], "novalue"))
      }
    }
  ]

  additional_build_groups_with_defaults = [
    for idx, ng in var.additional_build_groups : {
      id            = tonumber(lookup(ng, "id", idx))
      type          = lookup(ng, "type", null)
      types         = lookup(ng, "types", null)
      cpu           = tonumber(lookup(ng, "cpu", 0))
      mem           = tonumber(lookup(ng, "mem", 0))
      disk          = tonumber(lookup(ng, "disk", var.node_disk))
      capacity_type = lookup(ng, "capacity_type", "ON_DEMAND")
      min_size      = tonumber(lookup(ng, "min_size", 0))
      desired_size  = tonumber(lookup(ng, "min_size", 0))
      max_size      = tonumber(lookup(ng, "max_size", 100))
      label         = lookup(ng, "label", null)
      ami_id        = lookup(ng, "ami_id", null)
      tags = {
        for pair in compact(split(",", lookup(ng, "tags", ""))) :
        trimspace(split("=", pair)[0]) => trimspace(try(split("=", pair)[1], "novalue"))
      }
    }
  ]
}

resource "random_id" "additional_node_groups" {
  byte_length = 8

  for_each = { for idx, ng in local.additional_node_groups_with_defaults : ng.id => ng }

  keepers = {
    id                  = each.value.id
    node_capacity_type  = each.value.capacity_type != null ? each.value.capacity_type : "ON_DEMAND"
    node_disk           = each.value.disk != null ? each.value.disk : var.node_disk
    node_type           = each.value.type
    types               = each.value.types
    cpu                 = each.value.cpu
    mem                 = each.value.mem
    ami_id              = each.value.ami_id
    private             = var.private
    private_subnets_ids = join("-", local.private_subnets_ids)
    public_subnets_ids  = join("-", local.public_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
    tags                = try(jsonencode(each.value.tags), "")
  }
}

module "amitype" {
  source    = "../../helpers/aws"
  for_each  = { for ng in local.additional_node_groups_with_defaults : ng.id => ng }
  node_type = each.value.type
}

resource "aws_eks_node_group" "cluster_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  for_each = { for ng in local.additional_node_groups_with_defaults : ng.id => ng }

  ami_type        = random_id.additional_node_groups[each.key].keepers.ami_id != null ? "CUSTOM" : module.amitype[each.key].ami_type
  capacity_type   = random_id.additional_node_groups[each.key].keepers.node_capacity_type
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-additional-${each.key}-${random_id.additional_node_groups[each.key].hex}"
  node_role_arn   = random_id.additional_node_groups[each.key].keepers.role_arn
  subnet_ids      = var.private ? local.private_subnets_ids : local.public_subnets_ids
  tags            = each.value.tags == null ? local.tags : merge(local.tags, each.value.tags)
  version         = random_id.additional_node_groups[each.key].keepers.ami_id != null ? null : var.k8s_version

  launch_template {
    id      = aws_launch_template.cluster_additional[each.key].id
    version = "$Latest"
  }

  scaling_config {
    desired_size = each.value.desired_size != null ? each.value.desired_size : 1
    min_size     = each.value.min_size != null ? each.value.min_size : 1
    max_size     = each.value.max_size != null ? each.value.max_size : 100
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
    "convox.io/label" = each.value.label != null ? each.value.label : "custom"
  }

  dynamic "taint" {
    for_each = each.value.dedicated ? [1] : []
    content {
      key    = "dedicated-node"
      value  = each.value.label != null ? each.value.label : "custom"
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
  for_each = { for idx, ng in local.additional_node_groups_with_defaults : ng.id => ng }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = random_id.additional_node_groups[each.key].keepers.node_disk
      encrypted   = var.ebs_volume_encryption_enabled
    }
  }

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  dynamic "instance_requirements" {
    for_each = random_id.additional_node_groups[each.key].keepers.cpu > 0 && random_id.additional_node_groups[each.key].keepers.mem > 0 ? [1] : []
    content {
      vcpu_count { min = random_id.additional_node_groups[each.key].keepers.cpu }
      memory_mib { min = random_id.additional_node_groups[each.key].keepers.mem }
      allowed_instance_types = random_id.additional_node_groups[each.key].keepers.types != null ? random_id.additional_node_groups[each.key].keepers.types :
        random_id.additional_node_groups[each.key].keepers.node_type != null ? [random_id.additional_node_groups[each.key].keepers.node_type] : null
    }
  }

  instance_type = random_id.additional_node_groups[each.key].keepers.cpu > 0 && random_id.additional_node_groups[each.key].keepers.mem > 0 ? null : random_id.additional_node_groups[each.key].keepers.node_type

  image_id = random_id.additional_node_groups[each.key].keepers.ami_id

  dynamic "tag_specifications" {
    for_each = toset(
      concat(["instance", "volume", "network-interface", "spot-instances-request"],
        var.gpu_tag_enable ? ["elastic-gpu"] : []
    ))
    content {
      resource_type = tag_specifications.key
      tags          = each.value.tags == null ? local.tags : merge(local.tags, each.value.tags)
    }
  }

  user_data = random_id.additional_node_groups[each.key].keepers.ami_id == null ? null : base64encode(<<-EOF
#!/bin/bash
set -ex
/etc/eks/bootstrap.sh ${aws_eks_cluster.cluster.name} \
  --kubelet-extra-args '--node-labels=eks.amazonaws.com/nodegroup=${var.name}-additional-${each.key}-${random_id.additional_node_groups[each.key].hex}' \
  --b64-cluster-ca ${aws_eks_cluster.cluster.certificate_authority[0].data} \
  --apiserver-endpoint ${aws_eks_cluster.cluster.endpoint} --use-max-pods true --dns-cluster-ip ${local.kube_dns_ip}

${local.launch_template_user_data_raw}
EOF
  )
  key_name = var.key_pair_name != "" ? var.key_pair_name : null
}

module "asg_tags_cluster_additional" {
  source = "../../helpers/aws-asg-tag"

  for_each = { for idx, ng in local.additional_node_groups_with_defaults : ng.id => ng }

  asg_name = aws_eks_node_group.cluster_additional[each.key].resources[0].autoscaling_groups[0].name
  asg_tags = merge({
    "k8s.io/cluster-autoscaler/node-template/label/convox.io/label" = coalesce(each.value.label, "custom")
  }, coalesce(each.value.tags, {}))
}

###### additional build node groups

resource "random_id" "build_node_additional" {
  for_each = { for idx, ng in local.additional_build_groups_with_defaults : ng.id => ng }

  byte_length = 8

  keepers = {
    id                  = each.value.id
    node_disk           = each.value.disk != null ? each.value.disk : var.node_disk
    node_type           = each.value.type
    types               = each.value.types
    cpu                 = each.value.cpu
    mem                 = each.value.mem
    capacity_type       = each.value.capacity_type
    ami_id              = each.value.ami_id
    private_subnets_ids = join("-", local.private_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
    tags                = try(jsonencode(each.value.tags), "")
  }
}

module "build_amitype" {
  source    = "../../helpers/aws"
  for_each  = { for ng in local.additional_build_groups_with_defaults : ng.id => ng }
  node_type = each.value.type
}


resource "aws_eks_node_group" "build_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  for_each = { for idx, ng in local.additional_build_groups_with_defaults : ng.id => ng }

  ami_type        = random_id.build_node_additional[each.key].keepers.ami_id != null ? "CUSTOM" : module.build_amitype[each.key].ami_type
  capacity_type   = random_id.build_node_additional[each.key].keepers.capacity_type != null ? random_id.build_node_additional[each.key].keepers.capacity_type : "ON_DEMAND"
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-build-additional-${each.key}-${random_id.build_node_additional[each.key].hex}"
  node_role_arn   = random_id.build_node_additional[each.key].keepers.role_arn
  subnet_ids      = local.private_subnets_ids
  tags            = each.value.tags == null ? local.tags : merge(local.tags, each.value.tags)
  version         = random_id.build_node_additional[each.key].keepers.ami_id != null ? null : var.k8s_version

  labels = {
    "convox-build" : "true"
    "convox.io/label" = each.value.label != null ? each.value.label : "custom-build"
  }

  taint {
    key    = "dedicated"
    value  = "build"
    effect = "NO_SCHEDULE"
  }

  launch_template {
    id      = aws_launch_template.build_additional[each.key].id
    version = "$Latest"
  }

  scaling_config {
    desired_size = each.value.desired_size != null ? each.value.desired_size : 0
    min_size     = each.value.min_size != null ? each.value.min_size : 0
    max_size     = each.value.max_size != null ? each.value.max_size : 100
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_launch_template" "build_additional" {
  for_each = { for idx, ng in local.additional_build_groups_with_defaults : ng.id => ng }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = random_id.build_node_additional[each.key].keepers.node_disk
      encrypted   = var.ebs_volume_encryption_enabled
    }
  }

  dynamic "instance_requirements" {
    for_each = random_id.build_node_additional[each.key].keepers.cpu > 0 && random_id.build_node_additional[each.key].keepers.mem > 0 ? [1] : []
    content {
      vcpu_count { min = random_id.build_node_additional[each.key].keepers.cpu }
      memory_mib { min = random_id.build_node_additional[each.key].keepers.mem }
      allowed_instance_types = random_id.build_node_additional[each.key].keepers.types != null ? random_id.build_node_additional[each.key].keepers.types : [random_id.build_node_additional[each.key].keepers.node_type]
    }
  }

  instance_type = random_id.build_node_additional[each.key].keepers.cpu > 0 && random_id.build_node_additional[each.key].keepers.mem > 0 ? null : random_id.build_node_additional[each.key].keepers.node_type

  image_id = random_id.build_node_additional[each.key].keepers.ami_id

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  user_data = random_id.build_node_additional[each.key].keepers.ami_id == null ? null : base64encode(<<-EOF
#!/bin/bash
set -ex
/etc/eks/bootstrap.sh ${aws_eks_cluster.cluster.name} \
  --kubelet-extra-args '--node-labels=eks.amazonaws.com/nodegroup=${var.name}-build-additional-${each.key}-${random_id.build_node_additional[each.key].hex}' \
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
      tags          = each.value.tags == null ? local.tags : merge(local.tags, each.value.tags)
    }
  }

  key_name = var.key_pair_name != "" ? var.key_pair_name : null
}


module "asg_tags_build_additional" {
  source = "../../helpers/aws-asg-tag"

  for_each = { for idx, ng in local.additional_build_groups_with_defaults : ng.id => ng }

  asg_name = aws_eks_node_group.build_additional[each.key].resources[0].autoscaling_groups[0].name
  asg_tags = merge({
    "k8s.io/cluster-autoscaler/node-template/label/convox-build"    = "true"
    "k8s.io/cluster-autoscaler/node-template/label/convox.io/label" = coalesce(each.value.label, "custom-build")
  }, coalesce(each.value.tags, {}))
}

