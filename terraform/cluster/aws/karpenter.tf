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
    command = "echo 'Karpenter finalizer cleanup handled by API endpoint'"
  }
}

# Pre-disable cleanup — runs as a CREATE provisioner when karpenter_enabled
# transitions from true to false. Calls the rack API's Go-based cleanup endpoint
# which handles controller shutdown, finalizer stripping, and CRD instance deletion
# using in-cluster auth. EC2 instance termination runs as a safety net via AWS CLI.
resource "null_resource" "karpenter_pre_disable_cleanup" {
  count = var.karpenter_enabled ? 0 : (var.karpenter_auth_mode ? 1 : 0)

  triggers = {
    karpenter_disabled = "true"
  }

  provisioner "local-exec" {
    command = <<-CLEANUP
      echo "=== Karpenter pre-disable cleanup ==="
      NAMESPACE="${var.name}-system"

      # Generate kubeconfig using the SAME auth method as the TF providers.
      # The kubernetes/helm/kubectl providers use aws eks get-token with the
      # cluster endpoint directly. We replicate that here instead of using
      # aws eks update-kubeconfig (which failed in RC10 and RC11).
      export KUBECONFIG=/tmp/karpenter-cleanup-kubeconfig

      # Generate kubeconfig using SAME auth as TF providers (main.tf provider blocks).
      # Uses grouped echo instead of nested heredoc to avoid indentation issues.
      %{ if var.private_eks_host != "" }
      # Private EKS cluster — direct host with username/password
      {
        echo 'apiVersion: v1'
        echo 'kind: Config'
        echo 'clusters:'
        echo '- cluster:'
        echo "    server: ${var.private_eks_host}"
        echo '    insecure-skip-tls-verify: true'
        echo '  name: karpenter-cleanup'
        echo 'contexts:'
        echo '- context:'
        echo '    cluster: karpenter-cleanup'
        echo '    user: karpenter-cleanup'
        echo '  name: karpenter-cleanup'
        echo 'current-context: karpenter-cleanup'
        echo 'users:'
        echo '- name: karpenter-cleanup'
        echo '  user:'
        echo "    username: ${var.private_eks_user}"
        echo "    password: ${var.private_eks_pass}"
      } > "$KUBECONFIG"
      %{ else }
      # Public EKS cluster — exec-based token (same as TF provider block)
      {
        echo 'apiVersion: v1'
        echo 'kind: Config'
        echo 'clusters:'
        echo '- cluster:'
        echo "    server: ${aws_eks_cluster.cluster.endpoint}"
        echo "    certificate-authority-data: ${aws_eks_cluster.cluster.certificate_authority.0.data}"
        echo '  name: karpenter-cleanup'
        echo 'contexts:'
        echo '- context:'
        echo '    cluster: karpenter-cleanup'
        echo '    user: karpenter-cleanup'
        echo '  name: karpenter-cleanup'
        echo 'current-context: karpenter-cleanup'
        echo 'users:'
        echo '- name: karpenter-cleanup'
        echo '  user:'
        echo '    exec:'
        echo '      apiVersion: client.authentication.k8s.io/v1beta1'
        echo '      command: aws'
        echo '      args:'
        echo '      - eks'
        echo '      - get-token'
        echo '      - --cluster-name'
        echo "      - ${var.name}"
      } > "$KUBECONFIG"
      %{ endif }

      API_READY=false
      for attempt in 1 2 3 4 5 6; do
        if kubectl cluster-info --request-timeout=30s >/dev/null 2>&1; then
          echo "K8s API reachable (attempt $attempt/6)"
          API_READY=true
          break
        fi
        echo "K8s API unreachable (attempt $attempt/6), retrying in 15s..."
        sleep 15
      done

      if [ "$API_READY" = "true" ]; then
        # Get API password from deployment env
        API_PASSWORD=$(kubectl get deploy api -n "$NAMESPACE" \
          -o jsonpath='{.spec.template.spec.containers[?(@.name=="system")].env[?(@.name=="PASSWORD")].value}' 2>/dev/null)

        if [ -n "$API_PASSWORD" ]; then
          # Port-forward to rack API and call Go cleanup endpoint
          kubectl port-forward -n "$NAMESPACE" svc/api 15443:5443 >/dev/null 2>&1 &
          PF_PID=$!
          sleep 3

          echo "Calling rack API cleanup endpoint..."
          HTTP_CODE=$(curl -sk -X POST "https://localhost:15443/system/karpenter/cleanup" \
            -u "convox:$API_PASSWORD" \
            --max-time 300 --retry 2 --retry-delay 10 \
            -o /dev/null -w '%%{http_code}' 2>/dev/null) || true

          kill $PF_PID 2>/dev/null || true
          wait $PF_PID 2>/dev/null || true

          if [ "$HTTP_CODE" = "200" ]; then
            echo "Cleanup endpoint returned 200 OK"
          else
            echo "WARNING: Cleanup endpoint returned HTTP $HTTP_CODE (expected 200)."
            echo "Falling back to kubectl-based cleanup..."
            # Inline fallback for cross-version (404), partial failure (5xx), or any non-200
            kubectl delete deployment karpenter -n kube-system --timeout=30s 2>/dev/null || true
            kubectl delete pods -n kube-system -l app.kubernetes.io/name=karpenter --force --grace-period=0 2>/dev/null || true
            sleep 10
            for crd in nodeclaims.karpenter.sh nodepools.karpenter.sh ec2nodeclasses.karpenter.k8s.aws; do
              for resource in $(kubectl get "$crd" -o name 2>/dev/null); do
                kubectl patch "$resource" --type=merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
              done
              kubectl delete "$crd" --all --timeout=30s 2>/dev/null || true
            done
          fi
        else
          echo "WARNING: Could not read API password. Skipping API cleanup."
        fi
      else
        echo "WARNING: K8s API unreachable after 6 attempts. Skipping API cleanup."
      fi

      # EC2 safety net — ALWAYS runs regardless of API cleanup result
      echo "--- EC2 safety net: checking for orphaned Karpenter instances ---"
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

      echo "=== Karpenter pre-disable cleanup complete ==="
    CLEANUP
  }
}
