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

variable "cluster" {
  type = string
}

variable "convox_domain_tls_cert_disable" {
  default = false
  type    = bool
}

variable "convox_rack_domain" {
  default = ""
  type    = string
}

variable "custom_provided_bucket" {
  type    = string
  default = ""
}

variable "deploy_extra_nlb" {
  default = false
  type    = bool
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "disable_convox_resolver" {
  type    = bool
  default = false
}

variable "disable_image_manifest_cache" {
  type    = bool
  default = false
}

variable "ecr_additional_policy_arn" {
  type    = string
  default = ""
}

variable "ecr_full_access" {
  type    = bool
  default = false
}

variable "ecr_scan_on_push_enable" {
  type    = bool
  default = false
}

variable "ecr_docker_hub_cache_prefix" {
  type    = string
  default = ""
}

variable "efs_csi_driver_enable" {
  type    = bool
  default = false
}

variable "efs_file_system_id" {
  type = string
}

// for eks addons dependency
variable "eks_addons" {}

variable "high_availability" {
  default = true
}

variable "idle_timeout" {
  type = number
}

variable "internal_router" {
  type    = bool
  default = false
}

variable "contour_internal_tls" {
  type    = bool
  default = true
}

variable "image" {
  type = string
}

variable "cost_tracking_enable" {
  type    = bool
  default = false
}

variable "seccomp_default_enabled" {
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

variable "lbc_helm_id" {
  default = ""
  type    = string
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "nlb_security_group" {
  default = ""
  type    = string
}

variable "nginx_image" {
  type    = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.12.0@sha256:e6b8de175acda6ca913891f0f727bca4527e797d52688cbe9fec9040d6f6b6fa"
}

variable "nginx_additional_config" {
  description = "Comma-separated key=value pairs (e.g., 'key1=value1,key2=value2')"
  type        = string
  default     = ""
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

variable "proxy_protocol" {
  default = false
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
  description = "Release-watcher GC sweep interval (e.g. 5m, 30m). Range 60s-1h enforced by pkg/cli/rack.go validator. Plumbed into the api Deployment as RELEASE_WATCHER_GC_INTERVAL env var."
}

variable "gpu_metrics_max_pods" {
  type        = string
  default     = "100"
  description = "Max pods returned by the GPU metrics handler per request. Range 1-500 enforced by pkg/cli/rack.go validator. Plumbed into the api Deployment as GPU_METRICS_MAX_PODS env var; empty falls back to handler default 100."
}

variable "gpu_metrics_max_concurrent" {
  type        = string
  default     = "10"
  description = "Max concurrent GPU metrics QueryRange calls. Range 1-50 enforced by pkg/cli/rack.go validator. Plumbed into the api Deployment as GPU_METRICS_MAX_CONCURRENT env var; empty falls back to handler default 10."
}

variable "tags" {
  default = {}
}

variable "subnets" {
  type = list(any)
}

variable "ssl_ciphers" {
  default = ""
  type    = string
}

variable "ssl_protocols" {
  default = ""
  type    = string
}

variable "telemetry" {
  type = bool
}

variable "telemetry_map" {
  type = any
}

variable "telemetry_default_map" {
  type = any
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "router_type" {
  type    = string
  default = "nginx"
}

variable "cert_duration" {
  type    = string
  default = "2160h"
}

variable "contour_cpu_request" {
  type    = string
  default = "100m"
}

variable "contour_memory_request" {
  type    = string
  default = "128Mi"
}

variable "envoy_cpu_request" {
  type    = string
  default = "100m"
}

variable "envoy_memory_request" {
  type    = string
  default = "256Mi"
}
