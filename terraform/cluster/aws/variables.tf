variable "additional_node_groups" {
  type    = list(map(any))
  default = []
}

variable "additional_build_groups" {
  type    = list(map(any))
  default = []
}

variable "arm_type" {
  default = false
}

variable "availability_zones" {
  default = ""
}

variable "aws_ebs_csi_driver_version" {
  type    = string
  default = null
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

variable "enable_private_access" {
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

variable "ebs_volume_encryption_enabled" {
  type    = bool
  default = false
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

variable "karpenter_auth_mode" {
  type    = bool
  default = false
}

variable "karpenter_enabled" {
  type    = bool
  default = false
}

variable "karpenter_version" {
  type    = string
  default = "1.10.0"
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

variable "karpenter_arch" {
  type    = string
  default = "amd64"
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
  description = "JSON overrides for the workload NodePool and EC2NodeClass specs"
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

variable "additional_karpenter_nodepools" {
  type    = list(map(any))
  default = []
}

variable "keda_enable" {
  type    = bool
  default = false
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

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}

variable "node_type" {
  default = "t3.small"
}

variable "nvidia_device_plugin_enable" {
  default = false
}

variable "nvidia_device_time_slicing_replicas" {
  type    = number
  default = 0
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

variable "public_access_cidrs" {
  type    = list(string)
  default = ["0.0.0.0/0"]
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

variable "user_data" {
  type    = string
  default = ""
}

variable "user_data_url" {
  type    = string
  default = ""
}

variable "vpa_enable" {
  type    = bool
  default = false
}

variable "ecr_docker_hub_cache" {
  type    = bool
  default = false
}

variable "docker_hub_username" {
  type    = string
  default = ""
}

variable "docker_hub_password" {
  type    = string
  default = ""
}
