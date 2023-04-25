resource "kubernetes_namespace" "system" {
  metadata {
    labels = {
      app    = "system"
      rack   = var.name
      system = "convox"
      type   = "rack"
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
    name = "docker-hub-authentication"
  }

  data = {
    ".dockerconfigjson" = <<DOCKER
{
  "auths": {
    "https://index.docker.io/v2/": {
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

  data = jsondecode(file(var.telemetry_file))
}