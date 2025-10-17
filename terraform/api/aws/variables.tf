variable "disable_api_k8s_proxy" {
  default = false
  type    = bool
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
