
locals {
  launch_template_user_data_raw = var.user_data_url != "" || var.user_data != "" || local.kubelet_registry_set ? templatefile("${path.module}/files/custom_user_data.sh", {
    kubelet_registry_pull_qps = local.kubelet_registry_pull_qps_effective
    kubelet_registry_burst    = local.kubelet_registry_burst_effective
    kubelet_registry_set      = local.kubelet_registry_set
    fast_image_pull           = false
    user_data_script_file     = var.user_data_url != "" ? data.http.user_data_content[0].response_body : ""
    user_data                 = var.user_data
  }) : ""

  # Values outside int32 make kubelet exit, and a zero burst admits no pull at all. Zero QPS is
  # kubelet's own "no limit", which is why only the burst floor is 1.
  kubelet_registry_pull_qps_effective = min(2147483647, floor(max(0, var.kubelet_registry_pull_qps)))
  kubelet_registry_burst_effective    = min(2147483647, floor(max(1, var.kubelet_registry_burst)))

  # Derive the gate from the clamped values, never from the raw variables.
  kubelet_registry_set = local.kubelet_registry_pull_qps_effective != 5 || local.kubelet_registry_burst_effective != 10
  node_config_set      = local.kubelet_registry_set || var.fast_image_pull_enable

  node_config_user_data = local.node_config_set ? templatefile("${path.module}/files/custom_user_data.sh", {
    kubelet_registry_pull_qps = local.kubelet_registry_pull_qps_effective
    kubelet_registry_burst    = local.kubelet_registry_burst_effective
    kubelet_registry_set      = local.kubelet_registry_set
    fast_image_pull           = var.fast_image_pull_enable
    user_data_script_file     = ""
    user_data                 = ""
  }) : ""

  kube_dns_ip = cidrhost(aws_eks_cluster.cluster.kubernetes_network_config[0].service_ipv4_cidr, 10)

  # gp3 takes at most 500 IOPS per GiB and 0.25 MiB/s per IOPS, and floors at 3000 and 125, so
  # the pair resolves against each volume's own size. 1000 is the provider's ceiling, not AWS's.
  node_volume_iops_requested       = min(80000, floor(max(0, var.node_volume_iops)))
  node_volume_throughput_requested = min(1000, floor(max(0, var.node_volume_throughput)))

  node_volume_iops_effective       = local.node_volume_iops_requested > 0 ? max(3000, min(local.node_volume_iops_requested, var.node_disk * 500)) : 0
  node_volume_throughput_effective = local.node_volume_throughput_requested > 0 ? max(125, min(local.node_volume_throughput_requested, floor(max(3000, local.node_volume_iops_effective) * 0.25))) : 0

  additional_node_volume_iops = { for ng in local.additional_node_groups_with_defaults : ng.id =>
    local.node_volume_iops_requested > 0 ? max(3000, min(local.node_volume_iops_requested, ng.disk * 500)) : 0
  }
  additional_node_volume_throughput = { for ng in local.additional_node_groups_with_defaults : ng.id =>
    local.node_volume_throughput_requested > 0 ? max(125, min(local.node_volume_throughput_requested, floor(max(3000, local.additional_node_volume_iops[ng.id]) * 0.25))) : 0
  }

  additional_build_volume_iops = { for ng in local.additional_build_groups_with_defaults : ng.id =>
    local.node_volume_iops_requested > 0 ? max(3000, min(local.node_volume_iops_requested, ng.disk * 500)) : 0
  }
  additional_build_volume_throughput = { for ng in local.additional_build_groups_with_defaults : ng.id =>
    local.node_volume_throughput_requested > 0 ? max(125, min(local.node_volume_throughput_requested, floor(max(3000, local.additional_build_volume_iops[ng.id]) * 0.25))) : 0
  }

  additional_node_groups_with_defaults = [
    for idx, ng in var.additional_node_groups : {
      id            = tonumber(lookup(ng, "id", idx))
      type          = lookup(ng, "type", null)
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
    dummy               = "1"
    id                  = each.value.id
    node_capacity_type  = each.value.capacity_type != null ? each.value.capacity_type : "ON_DEMAND"
    node_disk           = each.value.disk != null ? each.value.disk : var.node_disk
    node_type           = each.value.type
    ami_id              = each.value.ami_id
    private             = var.private
    private_subnets_ids = join("-", local.private_subnets_ids)
    public_subnets_ids  = join("-", local.public_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
    tags                = try(jsonencode(each.value.tags), "")
  }

  lifecycle {
    ignore_changes = [keepers["tags"]]
  }
}

data "aws_ec2_instance_type" "additional_node_type" {
  for_each      = { for ng in local.additional_node_groups_with_defaults : ng.id => ng }
  instance_type = each.value.type
}

locals {
  additional_node_ami_type = {
    for key, inst in data.aws_ec2_instance_type.additional_node_type : key => (
      (substr(inst.instance_type, 0, 1) == "g" || substr(inst.instance_type, 0, 1) == "p" ||
      substr(inst.instance_type, 0, 3) == "inf" || substr(inst.instance_type, 0, 3) == "trn")
      ? "AL2023_x86_64_NVIDIA"
      : contains(inst.supported_architectures, "arm64") ? "AL2023_ARM_64_STANDARD" : "AL2023_x86_64_STANDARD"
    )
  }
}

resource "aws_eks_node_group" "cluster_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  for_each = { for ng in local.additional_node_groups_with_defaults : ng.id => ng }

  ami_type        = random_id.additional_node_groups[each.key].keepers.ami_id != null ? "CUSTOM" : local.additional_node_ami_type[each.key]
  capacity_type   = random_id.additional_node_groups[each.key].keepers.node_capacity_type
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-additional-${each.key}-${random_id.additional_node_groups[each.key].hex}"
  node_role_arn   = random_id.additional_node_groups[each.key].keepers.role_arn
  subnet_ids      = var.private ? local.private_subnets_ids : local.public_subnets_ids
  tags            = each.value.tags == null ? local.tags : merge(local.tags, each.value.tags)
  version         = random_id.additional_node_groups[each.key].keepers.ami_id != null ? null : var.k8s_version

  launch_template {
    id      = aws_launch_template.cluster_additional[each.key].id
    version = aws_launch_template.cluster_additional[each.key].latest_version
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
    update = var.terraform_update_timeout
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
      iops        = local.additional_node_volume_iops[each.key] > 0 ? local.additional_node_volume_iops[each.key] : null
      throughput  = local.additional_node_volume_throughput[each.key] > 0 ? local.additional_node_volume_throughput[each.key] : null
    }
  }

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  instance_type = random_id.additional_node_groups[each.key].keepers.node_type

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

  user_data = random_id.additional_node_groups[each.key].keepers.ami_id == null ? (
    local.node_config_user_data != "" ? base64encode(local.node_config_user_data) : null
    ) : base64encode(templatefile("${path.module}/files/custom_ami_userdata_al2023.sh", {
      api_server_endpoint       = aws_eks_cluster.cluster.endpoint,
      api_server_ca             = aws_eks_cluster.cluster.certificate_authority[0].data,
      name                      = aws_eks_cluster.cluster.name,
      cidr                      = var.cidr,
      cluster_dns               = local.kube_dns_ip,
      node_labels               = "eks.amazonaws.com/nodegroup=${var.name}-additional-${each.key}-${random_id.additional_node_groups[each.key].hex}",
      user_data                 = local.launch_template_user_data_raw,
      kubelet_registry_pull_qps = local.kubelet_registry_pull_qps_effective,
      kubelet_registry_burst    = local.kubelet_registry_burst_effective,
      kubelet_registry_set      = local.kubelet_registry_set,
      fast_image_pull           = var.fast_image_pull_enable,
  }))
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
    dummy               = "1"
    id                  = each.value.id
    node_disk           = each.value.disk != null ? each.value.disk : var.node_disk
    node_type           = each.value.type
    capacity_type       = each.value.capacity_type
    ami_id              = each.value.ami_id
    private_subnets_ids = join("-", local.private_subnets_ids)
    public_subnets_ids  = join("-", local.public_subnets_ids)
    role_arn            = local.build_minimal_role_enabled ? replace(aws_iam_role.karpenter_build_nodes[0].arn, "role/convox/", "role/") : replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
    tags                = try(jsonencode(each.value.tags), "")
  }

  lifecycle {
    ignore_changes = [keepers["tags"]]
  }
}

data "aws_ec2_instance_type" "build_node_type" {
  for_each      = { for ng in local.additional_build_groups_with_defaults : ng.id => ng }
  instance_type = each.value.type
}

locals {
  build_node_ami_type = {
    for key, inst in data.aws_ec2_instance_type.build_node_type : key => (
      (substr(inst.instance_type, 0, 1) == "g" || substr(inst.instance_type, 0, 1) == "p" ||
      substr(inst.instance_type, 0, 3) == "inf" || substr(inst.instance_type, 0, 3) == "trn")
      ? "AL2023_x86_64_NVIDIA"
      : contains(inst.supported_architectures, "arm64") ? "AL2023_ARM_64_STANDARD" : "AL2023_x86_64_STANDARD"
    )
  }
}

resource "aws_eks_node_group" "build_additional" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  for_each = { for idx, ng in local.additional_build_groups_with_defaults : ng.id => ng }

  ami_type        = random_id.build_node_additional[each.key].keepers.ami_id != null ? "CUSTOM" : local.build_node_ami_type[each.key]
  capacity_type   = random_id.build_node_additional[each.key].keepers.capacity_type != null ? random_id.build_node_additional[each.key].keepers.capacity_type : "ON_DEMAND"
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-build-additional-${each.key}-${random_id.build_node_additional[each.key].hex}"
  node_role_arn   = random_id.build_node_additional[each.key].keepers.role_arn
  subnet_ids      = var.private ? local.private_subnets_ids : local.public_subnets_ids
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
    version = aws_launch_template.build_additional[each.key].latest_version
  }

  scaling_config {
    desired_size = each.value.desired_size != null ? each.value.desired_size : 0
    min_size     = each.value.min_size != null ? each.value.min_size : 0
    max_size     = each.value.max_size != null ? each.value.max_size : 100
  }

  timeouts {
    update = var.terraform_update_timeout
    delete = "1h"
    create = "1h"
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
      iops        = local.additional_build_volume_iops[each.key] > 0 ? local.additional_build_volume_iops[each.key] : null
      throughput  = local.additional_build_volume_throughput[each.key] > 0 ? local.additional_build_volume_throughput[each.key] : null
    }
  }

  instance_type = random_id.build_node_additional[each.key].keepers.node_type

  image_id = random_id.build_node_additional[each.key].keepers.ami_id

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  user_data = random_id.build_node_additional[each.key].keepers.ami_id == null ? (
    local.node_config_user_data != "" ? base64encode(local.node_config_user_data) : null
    ) : base64encode(templatefile("${path.module}/files/custom_ami_userdata_al2023.sh", {
      api_server_endpoint       = aws_eks_cluster.cluster.endpoint,
      api_server_ca             = aws_eks_cluster.cluster.certificate_authority[0].data,
      name                      = aws_eks_cluster.cluster.name,
      cidr                      = var.cidr,
      cluster_dns               = local.kube_dns_ip,
      node_labels               = "eks.amazonaws.com/nodegroup=${var.name}-build-additional-${each.key}-${random_id.build_node_additional[each.key].hex}",
      user_data                 = "",
      kubelet_registry_pull_qps = local.kubelet_registry_pull_qps_effective,
      kubelet_registry_burst    = local.kubelet_registry_burst_effective,
      kubelet_registry_set      = local.kubelet_registry_set,
      fast_image_pull           = var.fast_image_pull_enable,
  }))

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

