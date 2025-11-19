data "aws_iam_policy_document" "assume_autoscaler" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = local.oidc_sub
      values   = ["system:serviceaccount:kube-system:cluster-autoscaler"]
    }

    principals {
      identifiers = [aws_iam_openid_connect_provider.cluster.arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "autoscaler" {
  name               = "${var.name}-autoscaler"
  assume_role_policy = data.aws_iam_policy_document.assume_autoscaler.json
  path               = "/convox/"
  tags               = local.tags
}

data "aws_iam_policy_document" "autoscale" {
  statement {
    actions = [
      "autoscaling:DescribeAutoScalingGroups",
      "autoscaling:DescribeAutoScalingInstances",
      "autoscaling:DescribeLaunchConfigurations",
      "autoscaling:DescribeTags",
      "autoscaling:DescribeScalingActivities",
      "autoscaling:SetDesiredCapacity",
      "autoscaling:TerminateInstanceInAutoScalingGroup",
      "ec2:DescribeLaunchTemplateVersions",
      "ec2:DescribeInstanceTypes",
      "ec2:DescribeImages",
      "ec2:GetInstanceTypesFromInstanceRequirements",
      "eks:DescribeNodegroup"
    ]
    effect    = "Allow"
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "autoscaler_autoscale" {
  name   = "autoscale"
  role   = aws_iam_role.autoscaler.name
  policy = data.aws_iam_policy_document.autoscale.json
}

locals {
  autoscaler_labels = {
    "k8s-addon" : "cluster-autoscaler.addons.k8s.io",
    "k8s-app" : "cluster-autoscaler",
  }
}

resource "kubernetes_service_account" "autoscaler" {
  depends_on = [
    null_resource.wait_eks_addons
  ]
  metadata {
    name      = "cluster-autoscaler"
    namespace = "kube-system"
    labels    = local.autoscaler_labels

    annotations = {
      "eks.amazonaws.com/role-arn" : aws_iam_role.autoscaler.arn,
    }
  }
}

resource "kubernetes_cluster_role" "autoscaler" {
  depends_on = [
    null_resource.wait_eks_addons
  ]
  metadata {
    name   = "cluster-autoscaler"
    labels = local.autoscaler_labels
  }

  rule {
    api_groups = [""]
    resources  = ["events", "endpoints"]
    verbs      = ["create", "patch"]
  }

  rule {
    api_groups = [""]
    resources  = ["pods/eviction"]
    verbs      = ["create"]
  }

  rule {
    api_groups = [""]
    resources  = ["pods/status"]
    verbs      = ["update"]
  }

  rule {
    api_groups     = [""]
    resources      = ["endpoints"]
    resource_names = ["cluster-autoscaler"]
    verbs          = ["get", "update"]
  }

  rule {
    api_groups = [""]
    resources  = ["nodes"]
    verbs      = ["watch", "list", "get", "create", "delete", "update"]
  }

  rule {
    api_groups = [""]
    resources  = ["namespaces", "pods", "services", "replicationcontrollers", "persistentvolumeclaims", "persistentvolumes"]
    verbs      = ["watch", "list", "get"]
  }

  rule {
    api_groups = ["batch"]
    resources  = ["jobs", "cronjobs"]
    verbs      = ["watch", "list", "get"]
  }

  rule {
    api_groups = ["batch", "extensions"]
    resources  = ["jobs"]
    verbs      = ["watch", "list", "get", "patch"]
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["replicasets", "daemonsets"]
    verbs      = ["watch", "list", "get"]
  }

  rule {
    api_groups = ["policy"]
    resources  = ["poddisruptionbudgets"]
    verbs      = ["watch", "list"]
  }

  rule {
    api_groups = ["apps"]
    resources  = ["statefulsets", "replicasets", "daemonsets"]
    verbs      = ["watch", "list", "get"]
  }

  rule {
    api_groups = ["discovery.k8s.io"]
    resources  = ["endpointslices"]
    verbs      = ["get", "list", "update", "create", "patch"]
  }

  rule {
    api_groups = ["storage.k8s.io"]
    resources  = ["csidrivers", "csinodes", "csistoragecapacities", "storageclasses", "volumeattachments"]
    verbs      = ["watch", "list", "get"]
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["coordination.k8s.io"]
    resources  = ["leases"]
    verbs      = ["create"]
  }

  rule {
    api_groups     = ["coordination.k8s.io"]
    resources      = ["leases"]
    resource_names = ["cluster-autoscaler"]
    verbs          = ["get", "update"]
  }
}

resource "kubernetes_cluster_role_binding" "autoscaler" {
  depends_on = [
    null_resource.wait_eks_addons
  ]
  metadata {
    name   = "cluster-autoscaler"
    labels = local.autoscaler_labels
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "cluster-autoscaler"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "cluster-autoscaler"
    namespace = "kube-system"
  }
}

resource "kubernetes_role" "autoscaler" {
  depends_on = [
    null_resource.wait_eks_addons
  ]
  metadata {
    name      = "cluster-autoscaler"
    namespace = "kube-system"
    labels    = local.autoscaler_labels
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["create", "list", "watch"]
  }

  rule {
    api_groups     = [""]
    resources      = ["configmaps"]
    resource_names = ["cluster-autoscaler-status", "cluster-autoscaler-priority-expander", "cluster-autoscaler"]
    verbs          = ["delete", "get", "update", "watch"]
  }

  rule {
    api_groups = ["coordination.k8s.io"]
    resources  = ["leases"]
    verbs      = ["create", "get", "update"]
  }
}

resource "kubernetes_role_binding" "autoscaler" {
  depends_on = [
    null_resource.wait_eks_addons
  ]
  metadata {
    name      = "cluster-autoscaler"
    namespace = "kube-system"
    labels    = local.autoscaler_labels
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = "cluster-autoscaler"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "cluster-autoscaler"
    namespace = "kube-system"
  }
}

resource "kubernetes_pod_disruption_budget_v1" "autoscaler" {
  metadata {
    name      = "cluster-autoscaler"
    namespace = "kube-system"
    labels    = local.autoscaler_labels
  }
  spec {
    min_available = "1"
    selector {
      match_labels = {
        "app" : "cluster-autoscaler"
      }
    }
  }
}

resource "kubernetes_deployment" "autoscaler" {
  depends_on = [
    aws_iam_role_policy.autoscaler_autoscale,
    null_resource.wait_eks_addons
  ]

  metadata {
    name      = "cluster-autoscaler"
    namespace = "kube-system"

    labels = {
      "app" : "cluster-autoscaler"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "app" : "cluster-autoscaler"
      }
    }

    template {
      metadata {
        labels = {
          "app" : "cluster-autoscaler"
        }

        annotations = {
          "prometheus.io/scrape" : "true",
          "prometheus.io/port" : "8085"
        }
      }

      spec {
        automount_service_account_token = true
        service_account_name            = "cluster-autoscaler"
        priority_class_name             = "system-node-critical"

        container {
          image             = "registry.k8s.io/autoscaling/cluster-autoscaler:v1.32.0"
          image_pull_policy = "IfNotPresent"
          name              = "cluster-autoscaler"

          command = [
            "./cluster-autoscaler",
            "--v=4",
            "--stderrthreshold=info",
            "--cloud-provider=aws",
            "--skip-nodes-with-local-storage=false",
            "--expander=least-waste",
            "--node-group-auto-discovery=asg:tag=k8s.io/cluster-autoscaler/enabled,k8s.io/cluster-autoscaler/${aws_eks_cluster.cluster.name}",
            "--balance-similar-node-groups",
            "--skip-nodes-with-system-pods=false",
            "--max-pod-eviction-time=5m",
          ]

          resources {
            limits = {
              cpu    = "100m"
              memory = "300Mi"
            }

            requests = {
              cpu    = "100m"
              memory = "300Mi"
            }
          }

          volume_mount {
            name       = "ssl-certs"
            mount_path = "/etc/ssl/certs/ca-certificates.crt"
            read_only  = "true"
          }
        }

        volume {
          name = "ssl-certs"

          host_path {
            path = "/etc/ssl/certs/ca-bundle.crt"
          }
        }
      }
    }
  }
}

resource "kubernetes_cluster_role" "hpa_external_metrics" {
  depends_on = [
    null_resource.wait_k8s_api
  ]
  metadata {
    name = "hpa-external-metrics"
  }

  rule {
    api_groups = ["external.metrics.k8s.io"]
    resources  = ["*"]
    verbs      = ["watch", "list", "get"]
  }
}

resource "kubernetes_cluster_role_binding" "hpa_external_metrics" {
  depends_on = [
    null_resource.wait_k8s_api
  ]
  metadata {
    name = "hpa-external-metrics"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "hpa-external-metrics"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "horizontal-pod-autoscaler"
    namespace = "kube-system"
  }
}
