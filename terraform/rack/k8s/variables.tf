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

// for eks addons dependency
variable "eks_addons" { # skipcq
  default = []
}

variable "name" {
  type = string
}

variable "release" {
  type = string
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

# Rack params whose plaintext value is a credential and must NOT live in
# the ConfigMap (which is plaintext in etcd and readable by anyone with
# cluster-admin). The values for these keys are written to
# kubernetes_secret.telemetry_redacted_params instead; the ConfigMap retains
# stub empty strings for backward compat with older callers reading the
# ConfigMap directly. Off-rack telemetry hashes the Secret values via
# SHA-256 before emission.
variable "redacted_param_keys" {
  type    = list(string)
  default = ["webhook_signing_key", "docker_hub_password", "private_eks_pass", "prometheus_url"]
}
