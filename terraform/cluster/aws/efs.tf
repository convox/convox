
// eks addon
resource "aws_eks_addon" "aws_efs_csi_driver" {
  depends_on = [
    null_resource.wait_k8s_api
  ]

  count = var.efs_csi_driver_enable ? 1 : 0

  cluster_name      = aws_eks_cluster.cluster.name
  addon_name        = "aws-efs-csi-driver"
  addon_version     = var.efs_csi_driver_version
  resolve_conflicts = "OVERWRITE"
}

// setup iam permissions
resource "aws_iam_role_policy" "efs_policy" {
  count = var.efs_csi_driver_enable ? 1 : 0

  name   = "aws-efs-csi-driver"
  role   = aws_iam_role.nodes.name
  policy = file("${path.module}/files/efs_driver_policy.json")
}

// setup efs file system
resource "aws_efs_file_system" "convox_efs" {
  count = var.efs_csi_driver_enable ? 1 : 0

  encrypted        = true
  performance_mode = "generalPurpose"
  tags             = local.tags
}

resource "aws_efs_mount_target" "efs_mount" {
  count = var.efs_csi_driver_enable ? length(local.private_subnets_ids) : 0

  file_system_id  = aws_efs_file_system.convox_efs[0].id
  subnet_id       = local.private_subnets_ids[count.index]
  security_groups = [aws_security_group.efs_security_group[0].id]
}

resource "aws_security_group" "efs_security_group" {
  count = var.efs_csi_driver_enable ? 1 : 0

  name        = "efs-${var.name}"
  description = "convox efs security group"
  vpc_id      = local.vpc_id

  tags = local.tags
}

resource "aws_security_group_rule" "allow_efs_2049" {
  count = var.efs_csi_driver_enable ? 1 : 0

  type                     = "ingress"
  security_group_id        = aws_security_group.efs_security_group[0].id
  source_security_group_id = aws_eks_cluster.cluster.vpc_config[0].cluster_security_group_id
  from_port                = 2049
  protocol                 = "tcp"
  to_port                  = 2049
}

// k8s storage class
// https://github.com/kubernetes-sigs/aws-efs-csi-driver/blob/master/examples/kubernetes/dynamic_provisioning/README.md
resource "kubernetes_storage_class_v1" "convox_efs" {
  depends_on = [null_resource.wait_k8s_api]

  count = var.efs_csi_driver_enable ? 1 : 0

  metadata {
    name = "efs-sc"
  }

  storage_provisioner = "efs.csi.aws.com"

  parameters = {
    provisioningMode      = "efs-ap"
    fileSystemId          = aws_efs_file_system.convox_efs[0].id
    directoryPerms        = "700"
    gidRangeStart         = "1000"                             # optional
    gidRangeEnd           = "20000"                            # optional
    basePath              = "/dp"                              # optional
    subPathPattern        = "$${.PVC.namespace}/$${.PVC.name}" # optional
    ensureUniqueDirectory = "false"                            # optional
    reuseAccessPoint      = "false"                            # optional
  }
}

resource "kubernetes_storage_class_v1" "convox_efs_775" {
  depends_on = [null_resource.wait_k8s_api]

  count = var.efs_csi_driver_enable ? 1 : 0

  metadata {
    name = "efs-sc-775"
  }

  storage_provisioner = "efs.csi.aws.com"

  parameters = {
    provisioningMode      = "efs-ap"
    fileSystemId          = aws_efs_file_system.convox_efs[0].id
    uid = "33"
    gid = "33"
    directoryPerms        = "0775"
    gidRangeStart         = "1000"                             # optional
    gidRangeEnd           = "20000"                            # optional
    basePath              = "/dp775"                              # optional
    subPathPattern        = "$${.PVC.namespace}/$${.PVC.name}" # optional
    ensureUniqueDirectory = "false"                            # optional
    reuseAccessPoint      = "false"                            # optional
  }
}

resource "kubernetes_storage_class_v1" "convox_efs_777" {
  depends_on = [null_resource.wait_k8s_api]

  count = var.efs_csi_driver_enable ? 1 : 0

  metadata {
    name = "efs-sc-777"
  }

  storage_provisioner = "efs.csi.aws.com"

  parameters = {
    provisioningMode      = "efs-ap"
    fileSystemId          = aws_efs_file_system.convox_efs[0].id
    uid = "1000"
    gid = "1000"
    directoryPerms        = "0775"
    gidRangeStart         = "1000"                             # optional
    gidRangeEnd           = "20000"                            # optional
    basePath              = "/dp777"                              # optional
    subPathPattern        = "$${.PVC.namespace}/$${.PVC.name}" # optional
    ensureUniqueDirectory = "false"                            # optional
    reuseAccessPoint      = "false"                            # optional
  }
}

resource "kubernetes_storage_class_v1" "convox_efs_base" {
  depends_on = [null_resource.wait_k8s_api]

  count = var.efs_csi_driver_enable ? 1 : 0

  metadata {
    name = "efs-sc-base"
  }

  storage_provisioner = "efs.csi.aws.com"

}

resource "kubernetes_persistent_volume_v1" "convox_efs_pv_root" {
  depends_on = [null_resource.wait_k8s_api]

  count = var.efs_csi_driver_enable ? 1 : 0

  metadata {
    name = "efs-pv-root"
  }

  spec {

    capacity = {
      storage = "200Gi"
    }

    access_modes = ["ReadWriteMany"]
    mount_options = [
      "noresvport",
      "rsize=1048576",
      "wsize=1048576",
      "hard",
      "timeo=600",
      "retrans=2",
    ]
    persistent_volume_reclaim_policy = "Retain"

    persistent_volume_source {
      csi {
        driver = "efs.csi.aws.com"
        volume_handle = "${aws_efs_file_system.convox_efs[0].id}"
      }
    }

    storage_class_name = kubernetes_storage_class_v1.convox_efs_base[0].metadata[0].name
    volume_mode = "Filesystem"
  }

}
