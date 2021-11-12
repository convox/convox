resource "kubernetes_service" "elasticsearch" {
  metadata {
    namespace = var.namespace
    name      = "elasticsearch"
  }

  spec {
    selector = {
      service = "elasticsearch"
    }

    port {
      name = "http"
      port = 9200
    }

    port {
      name = "transport"
      port = 9300
    }
  }
}

resource "kubernetes_config_map" "elasticsearch" {
  metadata {
    namespace = var.namespace
    name      = "elasticsearch"
  }

  data = {
    "elasticsearch.yml" = file("${path.module}/elasticsearch.yml")
    "ES_JAVA_OPTS"      = "-Xms512m -Xmx512m"
  }
}

resource "kubernetes_stateful_set" "elasticsearch" {
  metadata {
    namespace = var.namespace
    name      = "elasticsearch"
  }

  spec {
    service_name = "elasticsearch"
    replicas     = var.replicas

    selector {
      match_labels = {
        service = "elasticsearch"
      }
    }

    update_strategy {
      type = "RollingUpdate"
    }

    template {
      metadata {
        labels = {
          service = "elasticsearch"
        }
      }

      spec {
        security_context {
          fs_group = 1000
        }

        affinity {
          pod_anti_affinity {
            preferred_during_scheduling_ignored_during_execution {
              weight = 100
              pod_affinity_term {
                label_selector {
                  match_labels = {
                    system  = "convox"
                    service = "router"
                  }
                }
                topology_key = "kubernetes.io/hostname"
              }
            }
          }
        }

        init_container {
          name              = "init-sysctl"
          image             = "busybox"
          image_pull_policy = "IfNotPresent"
          command           = ["sysctl", "-w", "vm.max_map_count=262144"]

          security_context {
            privileged = true
          }
        }

        container {
          name  = "system"
          image = "docker.elastic.co/elasticsearch/elasticsearch:6.5.0"

          env {
            name = "ES_JAVA_OPTS"

            value_from {
              config_map_key_ref {
                name = "elasticsearch"
                key  = "ES_JAVA_OPTS"
              }
            }
          }

          port {
            name           = "http"
            container_port = 9200
          }

          port {
            name           = "transport"
            container_port = 9300
          }

          readiness_probe {
            http_get {
              scheme = "HTTP"
              path   = "/_cluster/health?local=true"
              port   = 9200
            }

            initial_delay_seconds = 5
          }

          resources {
            requests {
              memory = "1Gi"
            }
          }

          security_context {
            privileged  = true
            run_as_user = 1000

            capabilities {
              add = ["IPC_LOCK", "SYS_RESOURCE"]
            }
          }

          volume_mount {
            name       = "elasticsearch-config"
            mount_path = "/usr/share/elasticsearch/config/elasticsearch.yml"
            sub_path   = "elasticsearch.yml"
          }

          volume_mount {
            name       = "elasticsearch-data"
            mount_path = "/usr/share/elasticsearch/data"
          }
        }

        volume {
          name = "elasticsearch-config"

          config_map {
            name = "elasticsearch"

            items {
              key  = "elasticsearch.yml"
              path = "elasticsearch.yml"
            }
          }
        }
      }
    }

    volume_claim_template {
      metadata {
        namespace = var.namespace
        name      = "elasticsearch-data"
      }

      spec {
        access_modes = ["ReadWriteOnce"]

        resources {
          requests = {
            storage = "5Gi"
          }
        }
      }
    }
  }
}
