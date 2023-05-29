variable "buildkit_enabled" {
  default = false
}

variable "build_node_enabled" {
  default = false
  type    = bool
}

variable "cluster" {
  type = string
}

variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "ebs_csi_driver_name" {
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

variable "name" {
  type = string
}

variable "rack_name" {
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

variable "settings" {
  type    = string
  default = ""
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
