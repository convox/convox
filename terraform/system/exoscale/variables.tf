
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

variable "high_availability" {
  default = true
}

variable "k8s_version" {
  type = string
  default = "1.28.4"
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "image" {
  default = "convox/convox"
}

variable "instance_type" {
  type = string
  default = "standard.medium"
}

variable "zone" {
  type = string
  default = "ch-gva-2"
}

variable "release" {
  default = ""
}

variable "instance_disk_size" {
  type = number
  default = 50
}

variable "registry_disk" {
  type = string
  default = 50
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
  type    = bool
  default = false
}

variable "whitelist" {
  default = "0.0.0.0/0"
}
