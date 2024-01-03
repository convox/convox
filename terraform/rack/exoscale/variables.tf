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

variable "disable_image_manifest_cache" {
  type    = bool
  default = false
}


variable "high_availability" {
  default = true
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

variable "proxy_protocol" {
  default = false
}

variable "release" {
  type = string
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

variable "whitelist" {
  default = ["0.0.0.0/0"]
}

variable "zone" {
  type = string
  default = "ch-gva-2"
}

variable "registry_disk" {
  type = string
}
