# Karpenter Helm deployment (CRDs + controller)
# All resources gated on var.karpenter_enabled

# NOTE: This is the first OCI Helm release in the Convox codebase.
# All other helm_release resources use HTTPS repos. Karpenter uses
# oci://public.ecr.aws/karpenter — the Helm provider 2.12.1 supports this,
# and public ECR needs no authentication.

resource "helm_release" "karpenter_crd" {
  # CRDs are installed when auth_mode is enabled (one-way migration) and never
  # uninstalled. Helm CRD uninstall deadlocks on Karpenter finalizers — this is
  # a known upstream issue (aws/karpenter-provider-aws#6870). Empty CRDs with
  # no instances are harmless.
  count = var.karpenter_auth_mode ? 1 : 0

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
  timeout    = 600
}

resource "helm_release" "karpenter" {
  count = var.karpenter_enabled ? 1 : 0

  depends_on = [helm_release.karpenter_crd]

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

# Karpenter disable cleanup — drain wait + EC2 safety net.
#
# Architecture: When karpenter_enabled goes true→false, Terraform destroys
# resources in REVERSE dependency order. This resource exists while Karpenter
# is enabled and sits between kubectl_manifest resources and helm_release in
# the dependency chain. On destroy, its provisioner runs AFTER manifest DELETEs
# are sent (controller starts processing finalizers) but BEFORE the Helm release
# is uninstalled (controller stays alive during the wait).
#
# The destroy provisioner sleeps to give the controller time to:
#   1. Drain pods from Karpenter nodes (PDB-respecting eviction)
#   2. Terminate EC2 instances
#   3. Remove karpenter.sh/termination finalizers from NodeClaims
#   4. Process karpenter.k8s.aws/termination finalizers on EC2NodeClasses
#
# After the wait, the EC2 safety net terminates any remaining orphaned instances
# via the AWS API (which always works from the TF worker).
#
# Execution order on disable:
#   1. kubectl_manifest.* DESTROYED (DELETE sent to K8s API — 5-6s)
#   2. terraform_data.karpenter_drain DESTROYED (this provisioner: sleep + EC2 cleanup)
#   3. helm_release.karpenter DESTROYED (Helm uninstall kills controller)
resource "terraform_data" "karpenter_drain" {
  count = var.karpenter_enabled ? 1 : 0

  # Sit between Helm release and kubectl_manifests in the dependency chain.
  # kubectl_manifests depend on this → they're destroyed BEFORE this.
  # This depends on helm_release → helm_release is destroyed AFTER this.
  depends_on = [helm_release.karpenter]

  # Store values needed by the destroy provisioner (destroy provisioners
  # can only reference self, not other resources)
  input = {
    region       = data.aws_region.current.name
    cluster_name = aws_eks_cluster.cluster.name
  }

  provisioner "local-exec" {
    when    = destroy
    command = <<-CLEANUP
      echo "=== Karpenter drain wait ==="
      echo "Waiting 300s for Karpenter controller to process CRD finalizers..."
      echo "The controller is draining pods, terminating EC2 instances, and"
      echo "removing finalizers from NodeClaims and EC2NodeClasses."
      echo ""
      echo "Progress: kubectl_manifest DELETEs were just sent. The controller"
      echo "received them and is processing. Helm release (controller) will be"
      echo "destroyed after this wait completes."
      sleep 300
      echo "Drain wait complete."

      echo "--- EC2 safety net: checking for orphaned Karpenter instances ---"
      KARP_INSTANCES=$(aws ec2 describe-instances \
        --region "${self.output.region}" \
        --filters "Name=tag:karpenter.sh/nodepool,Values=*" \
                  "Name=tag:kubernetes.io/cluster/${self.output.cluster_name},Values=owned" \
                  "Name=instance-state-name,Values=running,pending" \
        --query 'Reservations[].Instances[].InstanceId' \
        --output text 2>/dev/null || true)
      if [ -n "$KARP_INSTANCES" ] && [ "$KARP_INSTANCES" != "None" ]; then
        echo "Terminating orphaned instances: $KARP_INSTANCES"
        aws ec2 terminate-instances \
          --region "${self.output.region}" \
          --instance-ids $KARP_INSTANCES 2>/dev/null || true
      else
        echo "No orphaned Karpenter instances found."
      fi
      echo "=== Karpenter cleanup complete ==="
    CLEANUP
  }
}
