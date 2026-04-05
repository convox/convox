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

# Finalizer cleanup — strips Karpenter finalizers before destroy to prevent
# CRD deletion deadlock. By depending on kubectl_manifest resources, this is
# destroyed BEFORE them (Terraform reverses dependency order on destroy).
# The controller is still running at this point, but we strip finalizers
# explicitly to guarantee no deadlock.
resource "null_resource" "karpenter_finalizer_cleanup" {
  count = var.karpenter_enabled ? 1 : 0

  depends_on = [
    kubectl_manifest.karpenter_nodepool_workload,
    kubectl_manifest.karpenter_ec2nodeclass_workload,
    kubectl_manifest.karpenter_nodepool_build,
    kubectl_manifest.karpenter_ec2nodeclass_build,
    kubectl_manifest.karpenter_nodepool_additional,
    kubectl_manifest.karpenter_ec2nodeclass_additional,
  ]

  provisioner "local-exec" {
    when    = destroy
    command = <<-CLEANUP
      echo "=== Karpenter cleanup: stopping controller then stripping finalizers ==="

      # The API container has kubectl with in-cluster auth via service account.
      if ! kubectl cluster-info --request-timeout=10s >/dev/null 2>&1; then
        echo "WARNING: Kubernetes API unreachable. Skipping cleanup."
        exit 0
      fi

      # Step 1: Kill the controller so it cannot re-add finalizers.
      # Scale to 0 AND delete the deployment entirely to prevent any reconciliation.
      echo "--- Stopping Karpenter controller ---"
      kubectl delete deployment karpenter -n kube-system --timeout=60s 2>/dev/null || true
      kubectl wait --for=delete pod -l app.kubernetes.io/name=karpenter -n kube-system --timeout=90s 2>/dev/null || true
      echo "--- Controller stopped ---"

      # Step 2: Strip finalizers and delete instances. Run in a loop because
      # Kubernetes foregroundDeletion finalizer can re-queue dependents.
      for attempt in 1 2 3; do
        echo "--- Cleanup pass $attempt ---"

        for nc in $(kubectl get nodeclaims.karpenter.sh -o name 2>/dev/null || true); do
          kubectl patch "$nc" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
        done

        for np in $(kubectl get nodepools.karpenter.sh -o name 2>/dev/null || true); do
          kubectl patch "$np" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
        done

        for ec2nc in $(kubectl get ec2nodeclasses.karpenter.k8s.aws -o name 2>/dev/null || true); do
          kubectl patch "$ec2nc" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
        done

        kubectl delete nodeclaims.karpenter.sh --all --timeout=30s 2>/dev/null || true
        kubectl delete nodepools.karpenter.sh --all --timeout=30s 2>/dev/null || true
        kubectl delete ec2nodeclasses.karpenter.k8s.aws --all --timeout=30s 2>/dev/null || true

        # Check if everything is gone
        REMAINING=$(kubectl get nodeclaims.karpenter.sh,nodepools.karpenter.sh,ec2nodeclasses.karpenter.k8s.aws --no-headers 2>/dev/null | wc -l || echo "0")
        if [ "$REMAINING" = "0" ]; then
          echo "--- All CRD instances removed ---"
          break
        fi
        echo "--- $REMAINING resources remaining, retrying in 5s ---"
        sleep 5
      done

      # Step 3: Strip finalizers from the CRDs themselves to unblock Helm uninstall
      echo "--- Stripping CRD finalizers ---"
      for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
        kubectl patch crd "$crd" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
      done

      echo "=== Karpenter cleanup complete ==="
    CLEANUP
  }
}
