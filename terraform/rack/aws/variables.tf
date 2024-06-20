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

variable "disable_image_manifest_cache" {
  type    = bool
  default = false
}

variable "ebs_csi_driver_name" {
  type = string
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

variable "oidc_arn" {
  type = string
}

variable "oidc_sub" {
  type = string
}

variable "proxy_protocol" {
  default = false
}

variable "release" {
  type = string
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
