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

variable "availability_zones" {
  default = ""
}

variable "aws_ebs_csi_driver_version" {
  type    = string
  default = "v1.41.0-eksbuild.1"
}

variable "build_disable_convox_resolver" {
  default = false
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
  default = "v1.11.4-eksbuild.2"
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

variable "fluentd_disable" {
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

variable "ecr_scan_on_push_enable" {
  type    = bool
  default = false
}

variable "efs_csi_driver_enable" {
  type    = bool
  default = false
}

variable "efs_csi_driver_version" {
  type    = string
  default = "v2.1.6-eksbuild.1"
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

variable "imds_tags_enable" {
  type    = bool
  default = false
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
  default = "v1.31.2-eksbuild.3"
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
  default = "1.31"
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

variable "node_type" {
  default = "t3.small"
}

variable "nginx_image" {
  type    = string
  default = "registry.k8s.io/ingress-nginx/controller:v1.12.0@sha256:e6b8de175acda6ca913891f0f727bca4527e797d52688cbe9fec9040d6f6b6fa"
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
  default = "v1.3.4-eksbuild.1"
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
  default = "v1.19.2-eksbuild.5"
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
