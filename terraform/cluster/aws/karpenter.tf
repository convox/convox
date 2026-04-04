# Karpenter Helm deployment (CRDs + controller)
# All resources gated on var.karpenter_enabled

# NOTE: This is the first OCI Helm release in the Convox codebase.
# All other helm_release resources use HTTPS repos. Karpenter uses
# oci://public.ecr.aws/karpenter — the Helm provider 2.12.1 supports this,
# and public ECR needs no authentication.

resource "helm_release" "karpenter_crd" {
  count = var.karpenter_enabled ? 1 : 0

  depends_on = [
    null_resource.wait_k8s_api,
    aws_iam_role.karpenter_controller,
    aws_iam_role_policy.karpenter_controller_ec2,
    aws_iam_role_policy.karpenter_controller_iam,
    aws_iam_role_policy.karpenter_controller_eks,
    aws_iam_role_policy.karpenter_controller_sqs,
    aws_iam_role_policy.karpenter_controller_pricing,
  ]

  name       = "karpenter-crd"
  namespace  = "kube-system"
  repository = "oci://public.ecr.aws/karpenter"
  chart      = "karpenter-crd"
  version    = var.karpenter_version
}

resource "helm_release" "karpenter" {
  count = var.karpenter_enabled ? 1 : 0

  depends_on = [helm_release.karpenter_crd]

  # Drain window: when karpenter_enabled goes true→false, NodePool manifests are
  # destroyed first (they depends_on this helm release, reversed during destroy).
  # This sleep gives the still-running controller time to process NodePool deletions
  # and terminate Karpenter-provisioned EC2 instances before the controller itself
  # is killed. Mirrors the iam.tf 300s destroy pattern.
  provisioner "local-exec" {
    when    = destroy
    command = "echo 'Waiting for Karpenter controller to drain nodes...' && sleep 300"
  }

  name       = "karpenter"
  namespace  = "kube-system"
  repository = "oci://public.ecr.aws/karpenter"
  chart      = "karpenter"
  version    = var.karpenter_version

  set {
    name  = "settings.clusterName"
    value = aws_eks_cluster.cluster.name
  }

  set {
    name  = "settings.interruptionQueue"
    value = aws_sqs_queue.karpenter_interruption[0].name
  }

  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = aws_iam_role.karpenter_controller[0].arn
  }

  set {
    name  = "replicas"
    value = var.high_availability ? 2 : 1
  }

  set {
    name  = "controller.resources.requests.cpu"
    value = "200m"
  }

  set {
    name  = "controller.resources.requests.memory"
    value = "256Mi"
  }

  # Pin controller to system nodes
  set {
    name  = "nodeSelector.convox\\.io/system-node"
    value = "true"
    type  = "string"
  }

  # Tolerate the system-node taint
  set {
    name  = "tolerations[0].key"
    value = "convox.io/system-node"
  }

  set {
    name  = "tolerations[0].operator"
    value = "Equal"
  }

  set {
    name  = "tolerations[0].value"
    value = "true"
    type  = "string"
  }

  set {
    name  = "tolerations[0].effect"
    value = "NoSchedule"
  }

  # Prevent Karpenter from evicting its own controller pods
  set {
    name  = "podAnnotations.karpenter\\.sh/do-not-disrupt"
    value = "true"
    type  = "string"
  }

  # Topology spread for HA — distribute controller replicas across nodes
  values = var.high_availability ? [yamlencode({
    topologySpreadConstraints = [{
      maxSkew           = 1
      topologyKey       = "kubernetes.io/hostname"
      whenUnsatisfiable = "DoNotSchedule"
      labelSelector = {
        matchLabels = {
          "app.kubernetes.io/name"     = "karpenter"
          "app.kubernetes.io/instance" = "karpenter"
        }
      }
    }]
  })] : []
}
