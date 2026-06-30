resource "random_string" "password" {
  length  = 64
  special = false
}

resource "kubernetes_secret_v1" "webhook_signing_key" {
  count = var.webhook_signing_key != "" ? 1 : 0

  metadata {
    name      = "webhook-signing-key"
    namespace = var.namespace
  }

  data = {
    value = var.webhook_signing_key
  }

  type = "Opaque"
}

resource "kubernetes_secret_v1" "prometheus_url" {
  count = var.prometheus_url != "" ? 1 : 0

  metadata {
    name      = "prometheus-url"
    namespace = var.namespace
  }

  data = {
    value = var.prometheus_url
  }

  type = "Opaque"
}

resource "kubernetes_secret_v1" "docker_hub_password" {
  count = var.docker_hub_password != "" ? 1 : 0

  metadata {
    name      = "docker-hub-password"
    namespace = var.namespace
  }

  data = {
    value = var.docker_hub_password
  }

  type = "Opaque"
}

resource "kubernetes_secret_v1" "api_password" {
  count = var.authentication ? 1 : 0

  metadata {
    name      = "api-password"
    namespace = var.namespace
  }

  data = {
    value = random_string.password.result
  }

  type = "Opaque"
}

resource "kubernetes_resource_quota" "gcp-critical-pods" {
  metadata {
    name      = "gcp-critical-pods"
    namespace = var.namespace
  }
  spec {
    hard = {
      pods = "1000"
    }
    scope_selector {
      match_expression {
        scope_name = "PriorityClass"
        operator   = "In"
        values     = ["system-node-critical", "system-cluster-critical"]
      }
    }
  }
}

resource "kubernetes_cluster_role" "api" {
  metadata {
    name = "${var.rack}-api"
  }

  rule {
    api_groups = ["*"]
    resources  = ["*"]
    verbs      = ["*"]
  }

  rule {
    non_resource_urls = ["*"]
    verbs             = ["*"]
  }
}

resource "kubernetes_cluster_role_binding" "api" {
  metadata {
    name = "${var.rack}-api"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.api.metadata.0.name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.api.metadata.0.name
    namespace = kubernetes_service_account.api.metadata.0.namespace
  }
}

resource "kubernetes_service_account" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    annotations = var.annotations
  }
}

resource "kubernetes_deployment" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    labels = {
      app     = "system"
      name    = "api"
      rack    = var.rack
      service = "api"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    min_ready_seconds      = 3
    revision_history_limit = 0
    replicas               = var.replicas

    selector {
      match_labels = {
        name    = "api"
        service = "api"
        system  = "convox"
        type    = "service"
      }
    }

    strategy {
      type = "RollingUpdate"
      rolling_update {
        max_surge       = 1
        max_unavailable = 0
      }
    }

    template {
      metadata {
        annotations = merge(var.annotations, {
          "convox.com/secret-checksum-webhook-signing-key" = sha256(var.webhook_signing_key)
          "convox.com/secret-checksum-prometheus-url"      = sha256(var.prometheus_url)
          "convox.com/secret-checksum-docker-hub-password" = sha256(var.docker_hub_password)
          "convox.com/secret-checksum-api-password"        = sha256(random_string.password.result)
        })

        labels = merge(var.labels, {
          app     = "system"
          name    = "api"
          rack    = var.rack
          service = "api"
          system  = "convox"
          type    = "service"
        })
      }

      spec {
        automount_service_account_token = true
        service_account_name            = kubernetes_service_account.api.metadata.0.name
        share_process_namespace         = true
        priority_class_name             = "system-cluster-critical"
        node_selector                   = var.karpenter_enabled ? { "convox.io/system-node" = "true" } : {}

        dynamic "toleration" {
          for_each = var.karpenter_enabled ? [1] : []
          content {
            key      = "convox.io/system-node"
            operator = "Equal"
            value    = "true"
            effect   = "NoSchedule"
          }
        }

        dynamic "image_pull_secrets" {
          for_each = var.docker_hub_authentication != "NULL" ? [var.docker_hub_authentication] : []
          content {
            name = var.docker_hub_authentication
          }
        }


        container {
          name              = "system"
          args              = ["api"]
          image             = "${var.image}:${var.release}"
          image_pull_policy = "IfNotPresent"

          dynamic "security_context" {
            for_each = var.system_readonly_rootfs_enabled ? [1] : []
            content {
              read_only_root_filesystem = true
            }
          }

          dynamic "env" {
            for_each = var.system_readonly_rootfs_enabled ? [1] : []
            content {
              name  = "HOME"
              value = "/tmp"
            }
          }

          env {
            name  = "BUILDKIT_ENABLED"
            value = var.buildkit_enabled
          }

          env {
            name  = "BUILD_NODE_ENABLED"
            value = var.build_node_enabled
          }

          env {
            name  = "BUILDKIT_HOST_PATH_CACHE_ENABLE"
            value = var.buildkit_host_path_cache_enable
          }

          env {
            name  = "CONVOX_DOMAIN_TLS_CERT_DISABLE"
            value = var.convox_domain_tls_cert_disable
          }

          env {
            name  = "DOCKER_HUB_USERNAME"
            value = var.docker_hub_username
          }

          dynamic "env" {
            for_each = var.docker_hub_password != "" ? [1] : []
            content {
              name = "DOCKER_HUB_PASSWORD"
              value_from {
                secret_key_ref {
                  name = kubernetes_secret_v1.docker_hub_password[0].metadata[0].name
                  key  = "value"
                }
              }
            }
          }

          env {
            name  = "ECR_DOCKER_HUB_CACHE_PREFIX"
            value = var.ecr_docker_hub_cache_prefix
          }

          env {
            name  = "COST_TRACKING_ENABLE"
            value = var.cost_tracking_enable ? "true" : "false"
          }

          env {
            name  = "DOMAIN"
            value = var.domain
          }

          env {
            name  = "DOMAIN_INTERNAL"
            value = var.domain_internal
          }

          env {
            name  = "DISABLE_IMAGE_MANIFEST_CACHE"
            value = var.disable_image_manifest_cache
          }


          env {
            name  = "IMAGE"
            value = "${var.image}:${var.release}"
          }

          env {
            name  = "METRICS_SCRAPER_HOST"
            value = var.metrics_scraper_host
          }

          env {
            name = "NAMESPACE"
            value_from {
              field_ref {
                field_path = "metadata.namespace"
              }
            }
          }

          dynamic "env" {
            for_each = var.authentication ? [1] : []
            content {
              name = "PASSWORD"
              value_from {
                secret_key_ref {
                  name = kubernetes_secret_v1.api_password[0].metadata[0].name
                  key  = "value"
                }
              }
            }
          }

          dynamic "env" {
            for_each = var.authentication ? [] : [1]
            content {
              name  = "PASSWORD"
              value = ""
            }
          }

          dynamic "env" {
            for_each = var.prometheus_url != "" ? [1] : []
            content {
              name = "PROMETHEUS_URL"
              value_from {
                secret_key_ref {
                  name = kubernetes_secret_v1.prometheus_url[0].metadata[0].name
                  key  = "value"
                }
              }
            }
          }

          env {
            name  = "RACK_NAME"
            value = var.rack_name
          }

          dynamic "env" {
            for_each = var.webhook_signing_key != "" ? [1] : []
            content {
              name = "WEBHOOK_SIGNING_KEY"
              value_from {
                secret_key_ref {
                  name = kubernetes_secret_v1.webhook_signing_key[0].metadata[0].name
                  key  = "value"
                }
              }
            }
          }

          env {
            name  = "VERSION"
            value = var.release
          }

          dynamic "env" {
            for_each = var.env

            content {
              name  = env.key
              value = env.value
            }
          }

          port {
            container_port = 5443
          }

          liveness_probe {
            http_get {
              path   = "/check"
              port   = 5443
              scheme = "HTTPS"
            }

            failure_threshold     = 5
            initial_delay_seconds = 0
            period_seconds        = 3
            success_threshold     = 1
            timeout_seconds       = 3
          }

          readiness_probe {
            http_get {
              path   = "/check"
              port   = 5443
              scheme = "HTTPS"
            }

            failure_threshold     = 5
            initial_delay_seconds = 0
            period_seconds        = 3
            success_threshold     = 1
            timeout_seconds       = 3
          }

          dynamic "volume_mount" {
            for_each = var.volumes
            iterator = volume

            content {
              name       = volume.key
              mount_path = volume.value
            }
          }

          dynamic "volume_mount" {
            for_each = var.system_readonly_rootfs_enabled ? { "tmp-dir" = "/tmp", "var-tmp-dir" = "/var/tmp" } : {}
            content {
              name       = volume_mount.key
              mount_path = volume_mount.value
            }
          }
        }

        volume {
          name = "docker"

          host_path {
            path = var.socket
          }
        }

        dynamic "volume" {
          for_each = var.volumes

          content {
            name = volume.key

            persistent_volume_claim {
              claim_name = "api-${volume.key}"
            }
          }
        }

        dynamic "volume" {
          for_each = var.system_readonly_rootfs_enabled ? { "tmp-dir" = "/tmp", "var-tmp-dir" = "/var/tmp" } : {}
          content {
            name = volume.key
            empty_dir {}
          }
        }

        dns_config {
          nameservers = [var.resolver]
        }

        dns_policy = "None"
      }
    }
  }
  depends_on = [
    kubernetes_resource_quota.gcp-critical-pods,
    kubernetes_secret_v1.webhook_signing_key,
    kubernetes_secret_v1.prometheus_url,
    kubernetes_secret_v1.docker_hub_password,
    kubernetes_secret_v1.api_password,
  ]
}

resource "kubernetes_service" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    port {
      name        = "https"
      port        = 5443
      target_port = 5443
      protocol    = "TCP"
    }

    port {
      name        = "kubernetes"
      port        = 8001
      target_port = 8001
      protocol    = "TCP"
    }

    selector = {
      system  = "convox"
      service = "api"
    }
  }
}

locals {
  ingress_annotations = var.router_type == "contour" ? {
    for k, v in var.annotations : k => v
    if !startswith(k, "cert-manager.io/")
  } : var.annotations
}

resource "kubernetes_ingress_v1" "api" {
  wait_for_load_balancer = true

  metadata {
    namespace = var.namespace
    name      = "api-ing-v1"

    annotations = merge({
      "convox.com/backend-protocol" : "https",
      "nginx.ingress.kubernetes.io/backend-protocol" : "https",
      "nginx.ingress.kubernetes.io/proxy-read-timeout" : "99999",
      "nginx.ingress.kubernetes.io/proxy-send-timeout" : "99999",
    }, local.ingress_annotations)

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["api.${var.domain}"]
      secret_name = "api-certificate"
    }

    rule {
      host = "api.${var.domain}"

      http {
        path {
          backend {
            service {
              name = kubernetes_service.api.metadata[0].name
              port {
                number = 5443
              }
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_ingress_v1" "kubernetes" {
  wait_for_load_balancer = true

  metadata {
    namespace = var.namespace
    name      = "kubernetes-ing-v1"

    annotations = merge({
      "nginx.ingress.kubernetes.io/use-regex" : "true",
    }, local.ingress_annotations)

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["api.${var.domain}"]
      secret_name = "api-certificate"
    }

    rule {
      host = "api.${var.domain}"

      http {
        path {
          path = "/kubernetes/.*"

          backend {
            service {
              name = kubernetes_service.api.metadata[0].name
              port {
                number = 8001
              }
            }
          }
        }
      }
    }
  }
}

resource "kubectl_manifest" "api_httpproxy" {
  count     = var.router_type == "contour" ? 1 : 0
  yaml_body = <<-YAML
    apiVersion: projectcontour.io/v1
    kind: HTTPProxy
    metadata:
      name: api
      namespace: ${var.namespace}
    spec:
      ingressClassName: contour
      virtualhost:
        fqdn: api.${var.domain}
        tls:
          secretName: api-certificate
      routes:
      - enableWebsockets: true
        timeoutPolicy:
          response: "infinity"
          idle: "infinity"
        services:
          - name: api
            port: 5443
            protocol: tls
      - conditions:
          - prefix: /kubernetes/
        enableWebsockets: true
        timeoutPolicy:
          response: "infinity"
          idle: "infinity"
        services:
          - name: api
            port: 8001
  YAML
}

resource "kubectl_manifest" "api_certificate" {
  count     = var.router_type == "contour" ? 1 : 0
  yaml_body = <<-YAML
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: api-certificate
      namespace: ${var.namespace}
    spec:
      secretName: api-certificate
      secretTemplate:
        labels:
          system: convox
          type: letsencrypt-certificate
      issuerRef:
        name: letsencrypt
        kind: ClusterIssuer
        group: cert-manager.io
      usages:
      - digital signature
      - key encipherment
      %{if var.cert_duration != ""}
      duration: ${var.cert_duration}
      %{endif}
      dnsNames:
      - api.${var.domain}
  YAML
}
