resource "kubernetes_namespace" "system" {
  metadata {
    labels = {
      app    = "system"
      rack   = var.name
      system = "convox"
      type   = "rack"
    }

    annotations = {
      "eks_addons_dependency" = length(var.eks_addons) > 0 ? var.eks_addons[0] : "" // explicit eks addon dependency
    }

    name = "${var.name}-system"
  }

  timeouts {
    delete = "30m"
  }
}

resource "kubernetes_config_map" "rack" {
  metadata {
    namespace = kubernetes_namespace.system.metadata[0].name
    name      = "rack"
  }

  data = {
    DOMAIN = var.domain
  }
}

resource "kubernetes_secret" "docker_hub_authentication" {
  count = var.docker_hub_username != "" ? 1 : 0
  metadata {
    namespace = kubernetes_namespace.system.metadata[0].name
    name      = "docker-hub-authentication"
  }

  data = {
    ".dockerconfigjson" = <<DOCKER
{
  "auths": {
    "https://index.docker.io/v1/": {
      "auth": "${base64encode("${var.docker_hub_username}:${var.docker_hub_password}")}"
    }
  }
}
DOCKER
  }

  type = "kubernetes.io/dockerconfigjson"
}

resource "kubernetes_config_map" "telemetry_configuration" {
  count = var.telemetry ? 1 : 0

  metadata {
    namespace = kubernetes_namespace.system.metadata[0].name
    name      = "telemetry-rack-params"
  }

  # Decision 8 — keys in redacted_param_keys are stubbed to empty
  # strings here. Real plaintext lives in
  # kubernetes_secret.telemetry_redacted_params and is overlaid by
  # the Go RackParams() consumer before SHA-256 hashing. Pre-D8
  # callers reading the ConfigMap directly see empty strings for
  # those keys (documented in 3.24.6 release notes).
  data = {
    for k, v in var.telemetry_map :
    k => contains(var.redacted_param_keys, k) ? "" : v
  }
}

resource "kubernetes_config_map" "telemetry_default_configuration" {
  count = var.telemetry ? 1 : 0

  metadata {
    namespace = kubernetes_namespace.system.metadata[0].name
    name      = "telemetry-default-rack-params"
  }

  data = var.telemetry_default_map
}

# Decision 8 — sidecar Secret holding the plaintext credential values
# for the keys listed in var.redacted_param_keys. Co-located with the
# telemetry ConfigMap so the Go RackParams() consumer can overlay the
# Secret values over the ConfigMap stubs before SHA-256 hashing for
# off-rack telemetry. Additive resource: pre-D8 racks have no Secret
# and the consumer falls back to ConfigMap values gracefully.
resource "kubernetes_secret" "telemetry_redacted_params" {
  count = var.telemetry ? 1 : 0

  metadata {
    namespace = kubernetes_namespace.system.metadata[0].name
    name      = "telemetry-rack-params-redacted"
  }

  type = "Opaque"

  data = {
    for k, v in var.telemetry_map :
    k => v if contains(var.redacted_param_keys, k)
  }
}
