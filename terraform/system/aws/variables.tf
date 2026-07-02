variable "access_log_retention_in_days" {
  default = "7"
}

variable "additional_node_groups_config" {
  type    = string
  default = ""
}

variable "additional_build_groups_config" {
  type    = string
  default = ""
}

variable "api_feature_gates" {
  type    = string
  default = ""
}

variable "availability_zones" {
  default = ""
}

variable "aws_ebs_csi_driver_version" {
  type    = string
  default = "v1.56.0-eksbuild.1"
}

variable "build_disable_convox_resolver" {
  default = false
}

variable "build_node_enabled" {
  default = false
  type    = bool
}

variable "build_node_minimal_role_enabled" {
  type    = bool
  default = false
}

variable "buildkit_host_path_cache_enable" {
  default = false
  type    = bool
}

variable "build_node_type" {
  default = ""
}

variable "build_node_min_count" {
  default = 0
}

variable "cert_duration" {
  default = "2160h"
  type    = string
}

variable "cidr" {
  default = "10.1.0.0/16"
}

variable "cost_tracking_enable" {
  type        = bool
  default     = false
  description = "Enable the rack-side cost accumulator and budget enforcement. Opt-in; false preserves existing behaviour."
}

variable "convox_domain_tls_cert_disable" {
  default = false
  type    = bool
}

variable "convox_rack_domain" {
  default = ""
  type    = string
}

// https://docs.aws.amazon.com/eks/latest/userguide/managing-coredns.html
variable "coredns_version" {
  type    = string
  default = "v1.13.2-eksbuild.1"
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

variable "ecr_docker_hub_cache" {
  type    = bool
  default = false
}

variable "fluentd_disable" {
  type    = bool
  default = false
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "disable_convox_resolver" {
  type    = bool
  default = false
}

variable "disable_image_manifest_cache" {
  type    = bool
  default = false
}

variable "disable_public_access" {
  type    = bool
  default = false
}

variable "enable_private_access" {
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

variable "ebs_volume_encryption_enabled" {
  type    = bool
  default = false
}

variable "efs_csi_driver_enable" {
  type    = bool
  default = false
}

variable "efs_csi_driver_version" {
  type    = string
  default = "v2.3.0-eksbuild.2"
}

variable "eks_api_server_public_access_cidrs" {
  type        = string
  description = "comma separated cidr"
  default     = "0.0.0.0/0"
}

variable "eks_api_server_private_access_cidrs" {
  type        = string
  description = "Comma-separated CIDRs allowed to access the EKS API via the private endpoint (TCP 443 on the cluster security group). Empty string = no rules added."
  default     = ""
}

variable "eks_log_types" {
  type        = string
  description = "Comma-separated EKS control plane log types to enable (api, audit, authenticator, controllerManager, scheduler). Empty = no logging."
  default     = ""
}

variable "gpu_tag_enable" {
  default = false
  type    = bool
}

variable "high_availability" {
  default = true
}

variable "idle_timeout" {
  type    = number
  default = 3600

  # validation {
  #   condition     = var.idle_timeout > 0 && var.idle_timeout < 4001
  #   error_message = "The idle_timeout must be a value between 1 and 4000."
  # }
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
  default = "convox/convox"
}

variable "imds_http_tokens" {
  type    = string
  default = "optional"
}

variable "imds_http_hop_limit" {
  type    = number
  default = 3
}

variable "imds_tags_enable" {
  type    = bool
  default = false
}

variable "internet_gateway_id" {
  default = ""
}

variable "eks_access_entries" {
  type    = string
  default = "false"
}

variable "karpenter_auth_mode" {
  type    = string
  default = "false"
}

variable "karpenter_enabled" {
  type    = string
  default = "false"
}

variable "karpenter_arch" {
  type    = string
  default = ""
}

variable "karpenter_instance_families" {
  type    = string
  default = ""
}

variable "karpenter_instance_sizes" {
  type    = string
  default = ""
}

variable "karpenter_capacity_types" {
  type    = string
  default = "on-demand"
}


variable "karpenter_cpu_limit" {
  type    = number
  default = 100
}

variable "karpenter_memory_limit_gb" {
  type    = number
  default = 400
}

variable "karpenter_consolidation_enabled" {
  type    = bool
  default = true
}

variable "karpenter_consolidate_after" {
  type    = string
  default = "30s"
}

variable "karpenter_node_expiry" {
  type    = string
  default = "720h"
}

variable "karpenter_disruption_budget_nodes" {
  type    = string
  default = "10%"
}

variable "karpenter_node_disk" {
  type    = number
  default = 0
}

variable "karpenter_node_volume_type" {
  type    = string
  default = "gp3"
}

variable "karpenter_node_labels" {
  type    = string
  default = ""
}

variable "karpenter_node_taints" {
  type    = string
  default = ""
}

variable "karpenter_config" {
  type    = string
  default = ""
}

variable "karpenter_build_instance_families" {
  type    = string
  default = ""
}

variable "karpenter_build_instance_sizes" {
  type    = string
  default = ""
}

variable "karpenter_build_capacity_types" {
  type    = string
  default = "on-demand"
}

variable "karpenter_build_cpu_limit" {
  type    = number
  default = 32
}

variable "karpenter_build_memory_limit_gb" {
  type    = number
  default = 256
}

variable "karpenter_build_consolidate_after" {
  type    = string
  default = "60s"
}

variable "karpenter_build_node_labels" {
  type    = string
  default = ""
}

variable "additional_karpenter_nodepools_config" {
  type    = string
  default = ""
}

variable "keda_enable" {
  type    = bool
  default = false
}

variable "key_pair_name" {
  type    = string
  default = ""
}

// https://docs.aws.amazon.com/eks/latest/userguide/managing-kube-proxy.html
variable "kube_proxy_version" {
  type    = string
  default = "v1.34.3-eksbuild.2"
}

variable "kubelet_registry_pull_qps" {
  type    = number
  default = 5
}

variable "kubelet_registry_burst" {
  type    = number
  default = 10
}

variable "k8s_version" {
  type    = string
  default = "1.34"
}

variable "max_on_demand_count" {
  default = 100
}

variable "min_on_demand_count" {
  default = 1
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

variable "node_capacity_type" {
  default = "on_demand"
}

variable "node_disk" {
  default = 20
}

variable "node_max_unavailable_percentage" {
  type    = number
  default = 0
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}

variable "node_type" {
  default = "t3.small"
}

variable "nginx_image" {
  type    = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.12.6@sha256:c371fbf42b4f23584ce879d99303463131f4f31612f0875482b983354eeca7e6"
}

variable "nginx_additional_config" {
  description = "Comma-separated key=value pairs (e.g., 'key1=value1,key2=value2')"
  type        = string
  default     = ""
}

variable "router_type" {
  type    = string
  default = "nginx"
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

variable "nvidia_device_plugin_enable" {
  default = false
}

variable "nvidia_device_time_slicing_replicas" {
  type    = number
  default = 0
}

variable "gpu_observability_enable" {
  type    = bool
  default = false
}

variable "gpu_observability_chart_version" {
  type    = string
  default = "4.8.1"
}

variable "pdb_default_min_available_percentage" {
  type    = number
  default = 50
}

variable "pod_identity_agent_enable" {
  type    = bool
  default = false
}

variable "pod_identity_agent_version" {
  type    = string
  default = "v1.3.10-eksbuild.2"
}

variable "private" {
  default = true
}

variable "private_subnets_ids" {
  type    = string
  default = ""
}

variable "prometheus_url" {
  type        = string
  default     = ""
  description = "External Prometheus URL for KEDA autoscale triggers and observability. User-set value enables GPU enrichment in `convox ps`. When empty (default), GPU fields show em-dash sentinels even when a chart is installed via the Convox Console. Set to the in-cluster service URL for Convox-Console-managed monitoring (paid: `http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090`; free: `http://prometheus-gpu-metrics-server.kube-system.svc.cluster.local:80`) or to your external Prometheus."
}

variable "prometheus_gpu_metrics_chart_version" {
  type    = string
  default = "27.9.0"
}

# User-facing Grafana base URL for the Console "Open in your Grafana"
# deep-link button. Empty default keeps the button inert (renders a hint
# instead of a link). Reconciler-safe: additive new variable with safe
# zero-value default; downgrade leaves the rack working.
variable "grafana_url" {
  type        = string
  default     = ""
  description = "User-facing Grafana base URL for the Console deep-link button. Optional. Empty default keeps the button inert."
}

variable "prometheus_gpu_metrics_retention" {
  type    = string
  default = "24h"
}

# DCGM exporter scrape interval. Read by the Console-installed Prometheus
# scrape config (Console writes the operator override into the Prometheus
# Helm values; rack TF declares the variable so the reconciler accepts it
# on upgrade and strips it cleanly on downgrade). Range 15s-300s; empty
# defaults to 15s.
variable "dcgm_scrape_interval" {
  type        = string
  default     = "15s"
  description = "DCGM exporter scrape interval (e.g. 15s, 30s). Empty defaults to 15s. Read by the Console-installed Prometheus scrape config; rack TF declares the variable so reconciler accepts it on upgrade and strips on downgrade."
}

# Per-service GPU time-range query handler caps. The rack-side handler bounds
# the number of pods returned and the number of concurrent Prom QueryRange
# calls to prevent a fan-out DoS from a busy app (many pods) or many
# simultaneous chart fetches. Plumbed system->rack->api->env map so the rack
# api Deployment surfaces GPU_METRICS_MAX_PODS / GPU_METRICS_MAX_CONCURRENT
# which the handler reads at request time. String-typed (matches
# release_watcher_gc_interval pattern) so empty/unset falls back to handler
# defaults cleanly without type coercion in the api/aws/main.tf env block.
variable "gpu_metrics_max_pods" {
  type        = string
  default     = "100"
  description = "Max pods returned by the GPU metrics handler per request. Range 1-500; default 100. Read by the handler at request time via GPU_METRICS_MAX_PODS env on the api Deployment."
}

variable "gpu_metrics_max_concurrent" {
  type        = string
  default     = "10"
  description = "Max concurrent GPU metrics QueryRange calls. Range 1-50; default 10. Read by the handler at request time via GPU_METRICS_MAX_CONCURRENT env on the api Deployment."
}

# Release-watcher GC sweep interval. Read once at provider Initialize from
# the RELEASE_WATCHER_GC_INTERVAL env var on the api Deployment; updates the
# package-level `releasePromoteWatchGCTickInterval` var which controls the
# periodic GC ticker. Range 60s-1h; empty defaults to 5m. Process-config.
variable "release_watcher_gc_interval" {
  type        = string
  default     = "5m"
  description = "Release-watcher GC sweep interval (e.g. 5m, 30m). Range 60s-1h; empty defaults to 5m. Read by the provider at Initialize via RELEASE_WATCHER_GC_INTERVAL env on the api Deployment."
}

# Grafana deep-link template variable name overrides. Operators with imported
# dashboards using non-default var names (e.g. `var-cluster_name` instead of
# `var-rack`) configure the substitutions here. Console reads from the rack
# params response and substitutes into Grafana URLs; absent → falls back to
# canonical defaults (rack/namespace/service/app). Process-config.
variable "grafana_dashboard_var_rack" {
  type        = string
  default     = "rack"
  description = "Grafana dashboard template variable name for the rack/cluster filter. Default 'rack'. Set if your imported dashboards use a different name (e.g. 'cluster_name')."
}

variable "grafana_dashboard_var_namespace" {
  type        = string
  default     = "namespace"
  description = "Grafana dashboard template variable name for the namespace filter. Default 'namespace'."
}

variable "grafana_dashboard_var_service" {
  type        = string
  default     = "service"
  description = "Grafana dashboard template variable name for the service filter. Default 'service'."
}

variable "grafana_dashboard_var_app" {
  type        = string
  default     = "app"
  description = "Grafana dashboard template variable name for the app filter. Default 'app'."
}

variable "public_subnets_ids" {
  type    = string
  default = ""
}

variable "proxy_protocol" {
  default = false
}

variable "private_eks_host" {
  default = ""
}

variable "private_eks_user" {
  default = ""
}

variable "private_eks_pass" {
  default = ""
}

variable "release" {
  default = ""
}

variable "releases_to_retain_after_active" {
  type    = number
  default = 0
}

variable "releases_to_retain_task_run_interval_hour" {
  type    = number
  default = 24
}

variable "region" {
  default = "us-east-1"
}

variable "schedule_rack_scale_down" {
  type    = string
  default = ""
}

variable "schedule_rack_scale_up" {
  type    = string
  default = ""
}

variable "settings" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "ssl_ciphers" {
  default = ""
  type    = string
}

variable "ssl_protocols" {
  default = ""
  type    = string
}

variable "tags" {
  default = ""
}

variable "telemetry" {
  type    = bool
  default = false
}

variable "user_data" {
  type    = string
  default = ""
}

variable "user_data_url" {
  type    = string
  default = ""
}

variable "vpc_id" {
  default = ""
}

// https://docs.aws.amazon.com/eks/latest/userguide/managing-vpc-cni.html
variable "vpc_cni_version" {
  type    = string
  default = "v1.21.1-eksbuild.3"
}

variable "vpa_enable" {
  type    = bool
  default = false
}

variable "webhook_signing_key" {
  type        = string
  default     = ""
  description = "Optional HMAC-SHA256 key(s) for signing outbound webhook payloads. Hex-encoded; comma-separated for rotation (max 4). When set, emits Convox-Signature header. Empty preserves 3.24.5 behavior (unsigned)."
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
