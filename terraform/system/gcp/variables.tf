variable "additional_node_groups_config" {
  type    = string
  default = ""
}

variable "buildkit_enabled" {
  default = false
}

variable "cert_duration" {
  default = "2160h"
  type    = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "image" {
  default = "convox/convox"
}

variable "k8s_version" {
  type    = string
  default = "1.35"
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "node_disk" {
  default = 100
}

variable "node_type" {
  default = "n1-standard-2"
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}

variable "nginx_additional_config" {
  description = "Comma-separated key=value pairs (e.g., 'key1=value1,key2=value2')"
  type        = string
  default     = ""
}

variable "preemptible" {
  default = true
}

variable "region" {
  default = "us-east1"
}

variable "release" {
  default = ""
}

variable "settings" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type    = bool
  default = false
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "webhook_signing_key" {
  type        = string
  default     = ""
  description = "Optional HMAC-SHA256 key(s) for signing outbound webhook payloads. Hex-encoded; comma-separated for rotation (max 2). When set, emits Convox-Signature header. Empty preserves 3.24.5 behavior (unsigned)."
}

variable "whitelist" {
  default = "0.0.0.0/0"
}

variable "gpu_observability_enable" {
  type        = bool
  default     = false
  description = "Install the DCGM exporter (NVIDIA GPU metrics on port 9400) and GPU Grafana dashboard ConfigMaps. GKE manages the device plugin. Metrics are exposed via pod annotations for a user-installed or Google Managed Prometheus to scrape."
}

variable "gpu_observability_chart_version" {
  type        = string
  default     = "4.8.1"
  description = "Pin the nvidia/dcgm-exporter Helm chart version. Default tracked by TestCoalesceLiteralsMatchTFDefaults."
}

variable "dcgm_scrape_interval" {
  type        = string
  default     = "15s"
  description = "Prometheus scrape interval hint set as a pod annotation on the DCGM exporter. Range 15s-300s enforced by pkg/cli/rack.go validator."
}

variable "private_api" {
  type    = bool
  default = false
}
