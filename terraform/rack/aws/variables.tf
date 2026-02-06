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

variable "ecr_scan_on_push_enable" {
  type    = bool
  default = false
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

variable "image" {
  type = string
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
