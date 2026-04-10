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
    null_resource.karpenter_crd_pre_destroy_cleanup, # Cleanup runs before CRD uninstall
  ]

  name       = "karpenter-crd"
  namespace  = "kube-system"
  repository = "oci://public.ecr.aws/karpenter"
  chart      = "karpenter-crd"
  version    = var.karpenter_version
  timeout    = 600
}

# Pre-CRD-destroy cleanup — runs as a CREATE provisioner when karpenter_auth_mode
# transitions from true to false. Follows the same pattern as
# karpenter_pre_disable_cleanup (line 155): CREATE-time provisioners have reliable
# dependency ordering, unlike destroy-time provisioners on null_resource.
# helm_release.karpenter_crd depends on this, so Terraform creates this
# (running cleanup) BEFORE destroying karpenter_crd.
resource "null_resource" "karpenter_crd_pre_destroy_cleanup" {
  count = var.karpenter_auth_mode ? 0 : 1

  triggers = {
    crd_cleanup = "true"
  }

  provisioner "local-exec" {
    command = <<-CLEANUP
      echo "=== Pre-CRD-destroy cleanup ==="

      export KUBECONFIG=/tmp/karpenter-crd-cleanup-kubeconfig
      aws eks update-kubeconfig \
        --name "${aws_eks_cluster.cluster.name}" \
        --region "${data.aws_region.current.name}" \
        --kubeconfig "$KUBECONFIG" 2>&1 || true

      if ! kubectl cluster-info --request-timeout=10s >/dev/null 2>&1; then
        echo "WARNING: Kubernetes API unreachable. Skipping CRD cleanup."
        exit 0
      fi

      # Check if any Karpenter CRD instances exist
      EXISTING=0
      for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
        COUNT=$(kubectl get "$crd" --no-headers 2>/dev/null | wc -l || echo "0")
        EXISTING=$((EXISTING + COUNT))
      done
      if [ "$EXISTING" = "0" ]; then
        echo "No Karpenter CRD instances found. Nothing to clean up."
        exit 0
      fi

      # Kill Karpenter controller if still running
      kubectl delete deployment karpenter -n kube-system --timeout=30s 2>/dev/null || true
      kubectl delete pods -n kube-system -l app.kubernetes.io/name=karpenter \
        --force --grace-period=0 2>/dev/null || true

      # Wait for controller to die
      DEAD=false
      for i in $(seq 1 60); do
        KARP_PODS=$(kubectl get pods -n kube-system -l app.kubernetes.io/name=karpenter \
          --no-headers 2>/dev/null | wc -l || echo "0")
        if [ "$KARP_PODS" = "0" ]; then
          echo "--- Controller confirmed dead after $${i}s ---"
          DEAD=true
          break
        fi
        if [ $((i % 10)) -eq 0 ]; then
          kubectl delete pods -n kube-system -l app.kubernetes.io/name=karpenter \
            --force --grace-period=0 2>/dev/null || true
        fi
        sleep 1
      done

      if [ "$DEAD" != "true" ]; then
        echo "WARNING: Controller pods still exist after 60s. Proceeding anyway."
      fi

      # Wait for in-flight reconciliation to settle
      sleep 15

      # Strip ALL instance finalizers and delete instances
      for attempt in 1 2 3 4 5; do
        echo "--- Cleanup pass $attempt ---"

        for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
          for resource in $(kubectl get "$crd" -o name 2>/dev/null); do
            kubectl patch "$resource" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
          done
          kubectl delete "$crd" --all --timeout=30s 2>/dev/null || true
        done

        REMAINING=0
        for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
          COUNT=$(kubectl get "$crd" --no-headers 2>/dev/null | wc -l || echo "0")
          REMAINING=$((REMAINING + COUNT))
        done
        if [ "$REMAINING" = "0" ]; then
          echo "--- All CRD instances removed ---"
          break
        fi
        echo "--- $REMAINING resources remaining, retrying in 5s ---"
        sleep 5
      done

      # Terminate orphaned EC2 instances
      echo "--- Terminating orphaned Karpenter EC2 instances ---"
      KARP_INSTANCES=$(aws ec2 describe-instances \
        --region "${data.aws_region.current.name}" \
        --filters "Name=tag:karpenter.sh/nodepool,Values=*" \
                  "Name=tag:kubernetes.io/cluster/${aws_eks_cluster.cluster.name},Values=owned" \
                  "Name=instance-state-name,Values=running,pending" \
        --query 'Reservations[].Instances[].InstanceId' \
        --output text 2>/dev/null || true)
      if [ -n "$KARP_INSTANCES" ] && [ "$KARP_INSTANCES" != "None" ]; then
        echo "Terminating orphaned instances: $KARP_INSTANCES"
        aws ec2 terminate-instances \
          --region "${data.aws_region.current.name}" \
          --instance-ids $KARP_INSTANCES 2>/dev/null || true
      else
        echo "No orphaned Karpenter instances found."
      fi

      # Strip CRD-level finalizers
      echo "--- Stripping CRD finalizers ---"
      for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
        kubectl patch crd "$crd" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
      done

      echo "=== Pre-CRD-destroy cleanup complete ==="
    CLEANUP
  }
}

resource "helm_release" "karpenter" {
  count = var.karpenter_enabled ? 1 : 0

  depends_on = [helm_release.karpenter_crd, null_resource.karpenter_pre_disable_cleanup]

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
    command = "echo 'Karpenter cleanup is handled by karpenter_pre_disable_cleanup resource'"
  }
}

# Pre-disable cleanup — runs as a CREATE provisioner when karpenter_enabled
# transitions from true to false. CREATE-time provisioners have reliable
# dependency ordering, unlike destroy-time provisioners on null_resource.
# kubectl_manifest resources depend on this, so Terraform creates this
# (running cleanup) BEFORE destroying kubectl_manifests (which becomes a no-op).
resource "null_resource" "karpenter_pre_disable_cleanup" {
  count = var.karpenter_enabled ? 0 : (var.karpenter_auth_mode ? 1 : 0)

  triggers = {
    karpenter_disabled = "true"
  }

  provisioner "local-exec" {
    command = <<-CLEANUP
      echo "=== Karpenter pre-disable cleanup ==="

      export KUBECONFIG=/tmp/karpenter-cleanup-kubeconfig
      aws eks update-kubeconfig \
        --name "${aws_eks_cluster.cluster.name}" \
        --region "${data.aws_region.current.name}" \
        --kubeconfig "$KUBECONFIG" 2>&1 || true

      if ! kubectl cluster-info --request-timeout=10s >/dev/null 2>&1; then
        echo "WARNING: Kubernetes API unreachable. Skipping cleanup."
        exit 0
      fi

      # Check if any Karpenter CRD instances exist
      EXISTING=$(kubectl get nodeclaims.karpenter.sh,nodepools.karpenter.sh,ec2nodeclasses.karpenter.k8s.aws --no-headers 2>/dev/null | wc -l || echo "0")
      if [ "$EXISTING" = "0" ]; then
        echo "No Karpenter CRD instances found. Nothing to clean up."
        exit 0
      fi

      # Verify controller is running — it MUST be alive to process finalizers
      KARP_PODS=$(kubectl get pods -n kube-system -l app.kubernetes.io/name=karpenter --no-headers 2>/dev/null | grep -c Running || echo "0")
      if [ "$KARP_PODS" = "0" ]; then
        echo "WARNING: Karpenter controller not running. Falling back to manual cleanup."
        # Fallback: strip finalizers manually and terminate EC2 directly
        # (same as current script but as a LAST RESORT, not the primary path)
        for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
          for resource in $(kubectl get "$crd" -o name 2>/dev/null); do
            kubectl patch "$resource" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
          done
          kubectl delete "$crd" --all --timeout=30s 2>/dev/null || true
        done
        # Terminate orphaned EC2 instances
        KARP_INSTANCES=$(aws ec2 describe-instances \
          --region "${data.aws_region.current.name}" \
          --filters "Name=tag:karpenter.sh/nodepool,Values=*" \
                    "Name=tag:kubernetes.io/cluster/${aws_eks_cluster.cluster.name},Values=owned" \
                    "Name=instance-state-name,Values=running,pending" \
          --query 'Reservations[].Instances[].InstanceId' \
          --output text 2>/dev/null || true)
        if [ -n "$KARP_INSTANCES" ] && [ "$KARP_INSTANCES" != "None" ]; then
          echo "Terminating orphaned instances: $KARP_INSTANCES"
          aws ec2 terminate-instances --region "${data.aws_region.current.name}" --instance-ids $KARP_INSTANCES 2>/dev/null || true
        fi
        exit 0
      fi

      echo "--- Controller is running ($KARP_PODS pods). Deleting NodePools to trigger graceful drain ---"

      # Step 1: Delete all NodePools. The controller will:
      #   - Drain nodes (evict pods via PDB-respecting eviction)
      #   - Terminate EC2 instances
      #   - Remove karpenter.sh/termination finalizer from NodeClaims
      #   - Delete NodeClaims
      kubectl delete nodepools.karpenter.sh --all --timeout=30s 2>/dev/null || true

      # Step 2: Wait for ALL NodeClaims to be gone (controller processes them)
      echo "--- Waiting for controller to process NodeClaim finalizers ---"
      for i in $(seq 1 180); do
        NC_COUNT=$(kubectl get nodeclaims.karpenter.sh --no-headers 2>/dev/null | wc -l || echo "0")
        if [ "$NC_COUNT" = "0" ]; then
          echo "--- All NodeClaims gone after $${i}s ---"
          break
        fi
        if [ $((i % 30)) -eq 0 ]; then
          echo "--- $NC_COUNT NodeClaims remaining ($i/180s) ---"
        fi
        sleep 1
      done

      # Check if NodeClaims are truly gone
      NC_REMAINING=$(kubectl get nodeclaims.karpenter.sh --no-headers 2>/dev/null | wc -l || echo "0")
      if [ "$NC_REMAINING" != "0" ]; then
        echo "WARNING: $NC_REMAINING NodeClaims still exist after 180s. Stripping finalizers manually."
        for nc in $(kubectl get nodeclaims.karpenter.sh -o name 2>/dev/null); do
          kubectl patch "$nc" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
        done
        kubectl delete nodeclaims.karpenter.sh --all --timeout=30s 2>/dev/null || true
      fi

      # Step 3: Delete EC2NodeClasses (controller processes karpenter.k8s.aws/termination finalizer)
      kubectl delete ec2nodeclasses.karpenter.k8s.aws --all --timeout=30s 2>/dev/null || true

      echo "--- Waiting for EC2NodeClasses to be removed ---"
      for i in $(seq 1 60); do
        EC2NC_COUNT=$(kubectl get ec2nodeclasses.karpenter.k8s.aws --no-headers 2>/dev/null | wc -l || echo "0")
        if [ "$EC2NC_COUNT" = "0" ]; then
          echo "--- All EC2NodeClasses gone after $${i}s ---"
          break
        fi
        sleep 1
      done

      EC2NC_REMAINING=$(kubectl get ec2nodeclasses.karpenter.k8s.aws --no-headers 2>/dev/null | wc -l || echo "0")
      if [ "$EC2NC_REMAINING" != "0" ]; then
        echo "WARNING: $EC2NC_REMAINING EC2NodeClasses still exist. Stripping finalizers."
        for ec2nc in $(kubectl get ec2nodeclasses.karpenter.k8s.aws -o name 2>/dev/null); do
          kubectl patch "$ec2nc" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
        done
        kubectl delete ec2nodeclasses.karpenter.k8s.aws --all --timeout=30s 2>/dev/null || true
      fi

      # Step 4: Safety net — terminate any remaining Karpenter EC2 instances
      # (should be zero if controller processed finalizers, but just in case)
      echo "--- Checking for orphaned EC2 instances ---"
      KARP_INSTANCES=$(aws ec2 describe-instances \
        --region "${data.aws_region.current.name}" \
        --filters "Name=tag:karpenter.sh/nodepool,Values=*" \
                  "Name=tag:kubernetes.io/cluster/${aws_eks_cluster.cluster.name},Values=owned" \
                  "Name=instance-state-name,Values=running,pending" \
        --query 'Reservations[].Instances[].InstanceId' \
        --output text 2>/dev/null || true)
      if [ -n "$KARP_INSTANCES" ] && [ "$KARP_INSTANCES" != "None" ]; then
        echo "Terminating remaining instances: $KARP_INSTANCES"
        aws ec2 terminate-instances --region "${data.aws_region.current.name}" --instance-ids $KARP_INSTANCES 2>/dev/null || true
      else
        echo "No orphaned instances."
      fi

      echo "=== Karpenter pre-disable cleanup complete ==="
    CLEANUP
  }
}
