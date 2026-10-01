variable "buildkit_enabled" {
  default = false
}

variable "cert_duration" {
  default = "2160h"
  type    = string
}

# defaults to the tenancy root, which is also where ocir repositories live
variable "compartment_ocid" {
  default = ""
  type    = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "fingerprint" {
  type = string
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "gpu_node_count" {
  default = 1
}

variable "gpu_node_type" {
  default = ""
  type    = string
}

variable "high_availability" {
  default = true
}

variable "image" {
  default = "convox/convox"
}

variable "k8s_version" {
  type    = string
  default = "1.35"
}

variable "name" {
  type = string
}

variable "node_count" {
  default = 2
}

variable "node_disk" {
  default = 100
}

variable "node_memory" {
  default = 16
}

variable "node_ocpus" {
  default = 2
}

variable "node_type" {
  default = "VM.Standard.E4.Flex"
}

variable "private_key" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "region" {
  default = "us-ashburn-1"
}

variable "release" {
  default = ""
}

variable "settings" {
  default = ""
}

variable "syslog" {
  default = ""
}

variable "telemetry" {
  type    = bool
  default = false
}

variable "tenancy_ocid" {
  type = string
}

variable "terraform_update_timeout" {
  type    = string
  default = "2h"
}

variable "user_ocid" {
  type = string
}

variable "webhook_signing_key" {
  type        = string
  default     = ""
  description = "Optional HMAC-SHA256 key(s) for signing outbound webhook payloads. Hex-encoded; comma-separated for rotation (max 2). When set, emits Convox-Signature header. Empty preserves 3.24.5 behavior (unsigned)."
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
