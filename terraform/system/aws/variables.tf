variable "access_log_retention_in_days" {
  default = "7"
}

variable "availability_zones" {
  default = ""
}

variable "build_node_enabled" {
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

// https://docs.aws.amazon.com/eks/latest/userguide/managing-coredns.html
variable "coredns_version" {
  type    = string
  default = "v1.9.3-eksbuild.2"
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
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

variable "internet_gateway_id" {
  default = ""
}

variable "key_pair_name" {
  type    = string
  default = ""
}

// https://docs.aws.amazon.com/eks/latest/userguide/managing-kube-proxy.html
variable "kube_proxy_version" {
  type    = string
  default = "v1.25.6-eksbuild.1"
}

variable "k8s_version" {
  type    = string
  default = "1.25"
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

variable "node_capacity_type" {
  default = "on_demand"
}

variable "node_disk" {
  default = 20
}

variable "node_type" {
  default = "t3.small"
}

variable "private" {
  default = true
}

variable "private_subnets_ids" {
  type    = string
  default = ""
}

variable "public_subnets_ids" {
  type    = string
  default = ""
}

variable "proxy_protocol" {
  default = false
}

variable "release" {
  default = ""
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

variable "vpc_id" {
  default = ""
}

// https://docs.aws.amazon.com/eks/latest/userguide/managing-vpc-cni.html
variable "vpc_cni_version" {
  type    = string
  default = "v1.12.6-eksbuild.2"
}

variable "whitelist" {
  default = "0.0.0.0/0"
}

