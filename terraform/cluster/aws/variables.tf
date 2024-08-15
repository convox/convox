variable "arm_type" {
  default = false
}

variable "availability_zones" {
  default = ""
}

variable "build_arm_type" {
  default = false
}

variable "build_gpu_type" {
  default = false
}

variable "build_node_enabled" {
  default = false
  type    = bool
}

variable "build_node_type" {
  type = string
}

variable "build_node_min_count" {
  default = 0
}

variable "cidr" {
  default = "10.1.0.0/16"
}

variable "coredns_version" {
  type    = string
  default = null
}

variable "disable_public_access" {
  type    = bool
  default = false
}

variable "efs_csi_driver_enable" {
  type    = bool
  default = false
}

variable "efs_csi_driver_version" {
  type    = string
  default = "v2.0.1-eksbuild.1"
}

variable "gpu_type" {
  default = false
}

variable "gpu_tag_enable" {
  default = false
  type    = bool
}

variable "high_availability" {
  default = true
}

variable "imds_http_tokens" {
  type    = string
  default = "optional"
}

variable "imds_http_hop_limit" {
  type    = number
  default = 2
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

variable "kube_proxy_version" {
  type    = string
  default = null
}

variable "kubelet_registry_pull_qps" {
  type = number
  default = 5
}

variable "kubelet_registry_burst" {
  type = number
  default = 10
}

variable "k8s_version" {
  type    = string
  default = "1.21"
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

variable "node_capacity_type" {
  default = "ON_DEMAND"
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

variable "pod_identity_agent_enable" {
  type    = bool
  default = false
}

variable "pod_identity_agent_version" {
  type    = string
  default = null
}

variable "private" {
  default = true
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

variable "schedule_rack_scale_down" {
  type    = string
  default = ""
}

variable "schedule_rack_scale_up" {
  type    = string
  default = ""
}

variable "tags" {
  default = {}
}

variable "vpc_id" {
  default = ""
}

variable "vpc_cni_version" {
  type    = string
  default = null
}

variable "private_subnets_ids" {
  type    = list(string)
  default = []
}

variable "public_subnets_ids" {
  type    = list(string)
  default = []
}
