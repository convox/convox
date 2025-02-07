provider "kubernetes" {
  cluster_ca_certificate = var.private_eks_host != "" ? null : base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
  host                   = var.private_eks_host != "" ? var.private_eks_host : aws_eks_cluster.cluster.endpoint

  dynamic "exec" {
    for_each = var.private_eks_host != "" ? [] : [1]
    content {
      api_version = "client.authentication.k8s.io/v1beta1"
      args        = ["eks", "get-token", "--cluster-name", var.name]
      command     = "aws"
    }
  }

  insecure = var.private_eks_host != "" ? true : false
  username = var.private_eks_host != "" ? var.private_eks_user : null
  password = var.private_eks_host != "" ? var.private_eks_pass : null
}

provider "helm" {
  kubernetes {
    cluster_ca_certificate = var.private_eks_host != "" ? null : base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
    host                   = var.private_eks_host != "" ? var.private_eks_host : aws_eks_cluster.cluster.endpoint

    dynamic "exec" {
      for_each = var.private_eks_host != "" ? [] : [1]
      content {
        api_version = "client.authentication.k8s.io/v1beta1"
        args        = ["eks", "get-token", "--cluster-name", var.name]
        command     = "aws"
      }
    }

    insecure = var.private_eks_host != "" ? true : false
    username = var.private_eks_host != "" ? var.private_eks_user : null
    password = var.private_eks_host != "" ? var.private_eks_pass : null
  }
}

locals {
  availability_zones     = var.availability_zones != "" ? compact(split(",", var.availability_zones)) : data.aws_availability_zones.available.names
  network_resource_count = var.high_availability ? 3 : 2
  oidc_sub               = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
  node_type_list = split(",", var.node_type)
  node_type_map  = zipmap(range(length(local.node_type_list)), local.node_type_list)
  build_node_type_list = split(",", var.build_node_type)
  build_node_type_map = zipmap(range(length(local.build_node_type_list)), local.build_node_type_list)
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_eks_cluster_auth" "cluster" {
  name = aws_eks_cluster.cluster.id
}

data "aws_region" "current" {}

resource "null_resource" "delay_cluster" {
  provisioner "local-exec" {
    command = "sleep 15"
  }

  triggers = {
    "eks_cluster" = aws_iam_role_policy_attachment.cluster_eks_cluster.id,
    "eks_service" = aws_iam_role_policy_attachment.cluster_eks_service.id,
  }
}

resource "aws_eks_cluster" "cluster" {
  depends_on = [
    null_resource.delay_cluster,
    null_resource.iam,
    null_resource.network,
  ]

  name     = var.name
  role_arn = aws_iam_role.cluster.arn
  tags     = local.tags
  version  = var.k8s_version

  vpc_config {
    endpoint_public_access  = var.disable_public_access ? false : true
    endpoint_private_access = var.disable_public_access
    security_group_ids      = [aws_security_group.cluster.id]
    subnet_ids              = concat(local.public_subnets_ids)
  }
}

resource "aws_iam_openid_connect_provider" "cluster" {
  client_id_list  = ["sts.amazonaws.com"]
  tags            = local.tags
  thumbprint_list = ["9e99a48a9960b14926bb7f3b02e22da2b0ab7280"]
  url             = aws_eks_cluster.cluster.identity.0.oidc.0.issuer
}

resource "random_id" "node_group" {
  for_each = local.node_type_map
  byte_length = 8

  keepers = {
    dummy               = "2"
    node_capacity_type  = var.node_capacity_type
    node_disk           = var.node_disk
    node_type           = each.value
    private             = var.private
    private_subnets_ids = join("-", local.private_subnets_ids)
    public_subnets_ids  = join("-", local.public_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

resource "random_id" "build_node_group" {
  for_each = local.build_node_type_map
  count = var.build_node_enabled ? 1 : 0

  byte_length = 8

  keepers = {
    dummy               = "2"
    node_disk           = var.node_disk
    node_type           = each.value
    private_subnets_ids = join("-", local.private_subnets_ids)
    role_arn            = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

module "amitype" {
  source    = "./amitype"
  for_each  = local.node_type_map
  node_type = each.value
}

module "amitype_build" {
  source    = "./amitype"
  for_each  = local.build_node_type_map
  node_type = each.value
}

resource "aws_eks_node_group" "cluster" {
  for_each = local.node_type_map

  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = var.high_availability ? 3 : 1

  ami_type        = module.amitype[each.key].gpu_type ? "AL2_x86_64_GPU" : module.amitype[each.key].arm_type ? "AL2_ARM_64" : "AL2_x86_64"
  capacity_type   = var.node_capacity_type == "MIXED" ? each.key == 0 ? "ON_DEMAND" : "SPOT" : var.node_capacity_type
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-${var.private ? data.aws_subnet.private_subnet_details[count.index].availability_zone : data.aws_subnet.public_subnet_details[count.index].availability_zone}-${count.index}${random_id.node_group[each.key].hex}"
  node_role_arn   = random_id.node_group[each.key].keepers.role_arn
  subnet_ids      = [var.private ? local.private_subnets_ids[count.index] : local.public_subnets_ids[count.index]]
  tags            = local.tags
  version         = var.k8s_version

  launch_template {
    id      = aws_launch_template.cluster[each.key].id
    version = "$Latest"
  }

  scaling_config {
    desired_size = each.key != 0 ? 0 : var.node_capacity_type == "MIXED" ? count.index == 0 ? var.min_on_demand_count : 1 : 1
    min_size     = each.key != 0 ? 0 : var.node_capacity_type == "MIXED" ? count.index == 0 ? var.min_on_demand_count : 1 : 1
    max_size     = var.node_capacity_type == "MIXED" ? count.index == 0 ? var.max_on_demand_count : 100 : 100
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

  timeouts {
    update = "2h"
    delete = "1h"
    create = "1h"
  }
}

resource "aws_eks_node_group" "cluster-build" {
  for_each = local.build_node_type_map

  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = var.build_node_enabled ? 1 : 0

  ami_type        = module.amitype_build[each.key].gpu_type ? "AL2_x86_64_GPU" : module.amitype_build[each.key].arm_type ? "AL2_ARM_64" : "AL2_x86_64"
  capacity_type   = "ON_DEMAND"
  cluster_name    = aws_eks_cluster.cluster.name
  node_group_name = "${var.name}-build-${data.aws_subnet.private_subnet_details[count.index].availability_zone}-${count.index}${random_id.build_node_group[each.key].hex}"
  node_role_arn   = random_id.build_node_group[each.key].keepers.role_arn
  subnet_ids      = [local.private_subnets_ids[count.index]]
  tags            = local.tags
  version         = var.k8s_version

  labels = {
    "convox-build" : "true"
  }

  taint {
    key    = "dedicated"
    value  = "build"
    effect = "NO_SCHEDULE"
  }

  launch_template {
    id      = aws_launch_template.cluster-build.id
    version = "$Latest"
  }

  scaling_config {
    desired_size = var.build_node_min_count
    min_size     = var.build_node_min_count
    max_size     = 100
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group_tag" "cluster-build" {
  depends_on = [
    aws_eks_node_group.cluster-build
  ]

  count = var.build_node_enabled ? 1 : 0

  autoscaling_group_name = aws_eks_node_group.cluster-build[0].resources[0].autoscaling_groups[0].name

  tag {
    key   = "k8s.io/cluster-autoscaler/node-template/label/convox-build"
    value = "true"

    propagate_at_launch = true
  }
}

// the cluster API takes some seconds to be available even when aws reports that the cluster is ready
// https://github.com/terraform-aws-modules/terraform-aws-eks/issues/621
resource "null_resource" "wait_k8s_api" {
  provisioner "local-exec" {
    command = "sleep 120 && echo ${aws_eks_node_group.cluster[0].id}"
  }

  depends_on = [
    aws_eks_cluster.cluster,
    aws_eks_node_group.cluster
  ]
}

resource "null_resource" "wait_k8s_cluster" {
  provisioner "local-exec" {
    command = "sleep 10"
  }

  depends_on = [
    aws_eks_node_group.cluster
  ]
}

resource "local_file" "kubeconfig" {
  depends_on = [
    aws_eks_node_group.cluster,
  ]

  filename = pathexpand("~/.kube/config.aws.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca       = aws_eks_cluster.cluster.certificate_authority.0.data
    cluster  = aws_eks_cluster.cluster.id
    endpoint = aws_eks_cluster.cluster.endpoint
  })
}

data "http" "user_data_content" {
  count = var.user_data_url != "" ? 1 : 0
  url = var.user_data_url
}

resource "aws_launch_template" "cluster" {
  for_each = local.node_type_map

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = random_id.node_group[each.key].keepers.node_disk
    }
  }

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  instance_type = each.value

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

  user_data = var.user_data_url != "" || var.user_data != "" || var.kubelet_registry_pull_qps != 5 || var.kubelet_registry_burst != 10 ? base64encode(templatefile("${path.module}/files/custom_user_data.sh",{
    kubelet_registry_pull_qps = var.kubelet_registry_pull_qps
    kubelet_registry_burst = var.kubelet_registry_burst
    user_data_script_file = var.user_data_url != "" ? data.http.user_data_content[0].body : ""
    user_data = var.user_data
  })) : ""

  key_name = var.key_pair_name != "" ? var.key_pair_name : null
}

resource "aws_launch_template" "cluster-build" {
  for_each = local.build_node_type_map

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = random_id.node_group.keepers.node_disk
    }
  }

  metadata_options {
    http_tokens                 = var.imds_http_tokens
    http_put_response_hop_limit = var.imds_http_hop_limit
    http_endpoint               = "enabled"
    instance_metadata_tags      = var.imds_tags_enable ? "enabled" : "disabled"
  }

  instance_type = each.value

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

module "ebs_csi_driver_controller" {
  depends_on = [
    null_resource.wait_eks_addons
  ]

  source = "github.com/convox/terraform-kubernetes-ebs-csi-driver?ref=01740b559d14f489e5ea2160d2dad0ee951fb4d9"

  arn_format                                 = data.aws_partition.current.partition
  ebs_csi_controller_image                   = "public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver"
  ebs_csi_driver_version                     = "v1.34.0"
  ebs_csi_controller_role_name               = "convox-ebs-csi-driver-controller"
  ebs_csi_controller_role_policy_name_prefix = "convox-ebs-csi-driver-policy"
  csi_controller_tolerations = [
    { operator = "Exists", key = "CriticalAddonsOnly" },
    { operator = "Exists", effect = "NoExecute", toleration_seconds = 300 }
  ]
  node_tolerations = [
    { operator = "Exists", key = "CriticalAddonsOnly" },
    { operator = "Exists", effect = "NoExecute", toleration_seconds = 300 }
  ]
  oidc_url = aws_iam_openid_connect_provider.cluster.url
}

resource "kubernetes_storage_class" "default" {
  depends_on = [
    null_resource.wait_k8s_api
  ]

  metadata {
    labels = {
      "ebs_driver_name" = module.ebs_csi_driver_controller.ebs_csi_driver_name
    }

    name = "gp3"
    annotations = {
      "storageclass.kubernetes.io/is-default-class" = "true"
    }
  }

  storage_provisioner    = "ebs.csi.aws.com"
  volume_binding_mode    = "WaitForFirstConsumer"
  allow_volume_expansion = true
  parameters = {
    type = "gp3"
  }
}

resource "kubernetes_annotations" "gp2" {
  depends_on = [
    kubernetes_storage_class.default
  ]

  api_version = "storage.k8s.io/v1"
  kind        = "StorageClass"

  metadata {
    name = "gp2"
  }

  annotations = {
    "storageclass.kubernetes.io/is-default-class" = "false"
  }

  force = true
}

resource "aws_eks_addon" "vpc_cni" {
  depends_on = [
    null_resource.wait_k8s_api
  ]

  cluster_name      = aws_eks_cluster.cluster.name
  addon_name        = "vpc-cni"
  addon_version     = var.vpc_cni_version
  resolve_conflicts = "OVERWRITE"
}

resource "aws_eks_addon" "coredns" {
  depends_on = [
    null_resource.wait_k8s_api
  ]

  cluster_name      = aws_eks_cluster.cluster.name
  addon_name        = "coredns"
  addon_version     = var.coredns_version
  resolve_conflicts = "OVERWRITE"
}

resource "aws_eks_addon" "kube_proxy" {
  depends_on = [
    null_resource.wait_k8s_api
  ]

  cluster_name      = aws_eks_cluster.cluster.name
  addon_name        = "kube-proxy"
  addon_version     = var.kube_proxy_version
  resolve_conflicts = "OVERWRITE"
}

resource "aws_eks_addon" "eks_pod_identity_agent" {
  depends_on = [
    null_resource.wait_k8s_api
  ]

  count = var.pod_identity_agent_enable ? 1 : 0

  cluster_name      = aws_eks_cluster.cluster.name
  addon_name        = "eks-pod-identity-agent"
  addon_version     = var.pod_identity_agent_version
  resolve_conflicts = "OVERWRITE"
}

resource "null_resource" "wait_eks_addons" {
  provisioner "local-exec" {
    command = "sleep 1"
  }

  depends_on = [
    aws_eks_addon.vpc_cni,
    aws_eks_addon.coredns,
    aws_eks_addon.kube_proxy
  ]
}

resource "aws_autoscaling_schedule" "scaledown" {
  count = length(var.schedule_rack_scale_down) > 6 ? (var.high_availability ? 3 : 1) : 0

  scheduled_action_name  = "scaledown${count.index}"
  min_size               = 0
  max_size               = 0
  desired_capacity       = 0
  recurrence             = var.schedule_rack_scale_down
  time_zone              = "UTC"
  autoscaling_group_name = flatten(aws_eks_node_group.cluster[count.index].resources[*].autoscaling_groups[*].name)[0]
}

resource "aws_autoscaling_schedule" "scaleup" {
  count = length(var.schedule_rack_scale_up) > 6 ? (var.high_availability ? 3 : 1) : 0

  scheduled_action_name  = "scaleup${count.index}"
  min_size               = 1
  max_size               = 100
  desired_capacity       = 1
  recurrence             = var.schedule_rack_scale_up
  time_zone              = "UTC"
  autoscaling_group_name = flatten(aws_eks_node_group.cluster[count.index].resources[*].autoscaling_groups[*].name)[0]
}
