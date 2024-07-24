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

variable "docker_hub_authentication" {
  type = string
}

variable "domain" {
  type = string
}

variable "domain_internal" {
  type = string
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

