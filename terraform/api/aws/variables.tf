variable "api_feature_gates" {
  type    = string
  default = ""
}

variable "buildkit_enabled" {
  default = false
}

variable "build_disable_convox_resolver" {
  default = false
}

variable "build_node_enabled" {
  default = false
  type    = bool
}

variable "buildkit_host_path_cache_enable" {
  default = false
  type    = bool
}

variable "cert_duration" {
  default = "2160h"
  type    = string
}

variable "convox_domain_tls_cert_disable" {
  default = false
  type    = bool
}

variable "custom_provided_bucket" {
  type    = string
  default = ""
}

variable "docker_hub_authentication" {
  type = string
}

variable "docker_hub_username" {
  type    = string
  default = ""
}

variable "docker_hub_password" {
  type    = string
  default = ""
}

variable "ecr_docker_hub_cache_prefix" {
  type    = string
  default = ""
}

variable "domain" {
  type = string
}

variable "domain_internal" {
  type = string
}

variable "disable_convox_resolver" {
  type    = bool
  default = false
}

variable "disable_image_manifest_cache" {
  type    = bool
  default = false
}

variable "ecr_scan_on_push_enable" {
  type    = bool
  default = false
}

variable "efs_file_system_id" {
  type = string
}

variable "efs_csi_driver_enable" {
  type    = bool
  default = false
}

variable "high_availability" {
  default = true
}

variable "image" {
  type = string
}

variable "cost_tracking_enable" {
  type    = bool
  default = false
}

variable "karpenter_enabled" {
  type    = bool
  default = false
}

variable "keda_enable" {
  type    = bool
  default = false
}

variable "metrics_scraper_host" {
  default = ""
  type    = string
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "oidc_arn" {
  type = string
}

variable "oidc_sub" {
  type = string
}

variable "pdb_default_min_available_percentage" {
  type    = number
  default = 50
}

variable "prometheus_url" {
  type    = string
  default = ""
}

variable "effective_prometheus_url" {
  type    = string
  default = ""
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "release" {
  type = string
}

variable "releases_to_retain_after_active" {
  type    = number
  default = 0
}

variable "releases_to_retain_task_run_interval_hour" {
  type    = number
  default = 24
}

variable "release_watcher_gc_interval" {
  type        = string
  default     = "5m"
  description = "Release-watcher GC sweep interval (e.g. 5m, 30m). Range 60s-1h enforced by pkg/cli/rack.go validator. Becomes RELEASE_WATCHER_GC_INTERVAL env var on the api Deployment via the env map at api/aws/main.tf."
}

variable "gpu_metrics_max_pods" {
  type        = string
  default     = "100"
  description = "Max pods returned by the GPU metrics handler per request. Range 1-500 enforced by pkg/cli/rack.go validator. Becomes GPU_METRICS_MAX_PODS env var on the api Deployment via the env map at api/aws/main.tf; empty falls back to handler default 100."
}

variable "gpu_metrics_max_concurrent" {
  type        = string
  default     = "10"
  description = "Max concurrent GPU metrics QueryRange calls. Range 1-50 enforced by pkg/cli/rack.go validator. Becomes GPU_METRICS_MAX_CONCURRENT env var on the api Deployment via the env map at api/aws/main.tf; empty falls back to handler default 10."
}

variable "resolver" {
  type = string
}

variable "router" {
  type = string
}

variable "subnets" {
  type = list(any)
}

variable "vpc_id" {
  type = string
}

variable "vpa_enable" {
  type    = bool
  default = false
}

variable "webhook_signing_key" {
  type    = string
  default = ""
}
