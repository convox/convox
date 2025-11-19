variable "docker_hub_authentication" {
  type    = string
  default = null
}

variable "env" {
  default = {}
}

variable "name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "network" {
  type = string
}

variable "nginx_additional_config" {
  description = "Comma-separated key=value pairs (e.g., 'key1=value1,key2=value2')"
  type        = string
  default     = ""
}

variable "release" {
  type = string
}

variable "whitelist" {
  default = ["0.0.0.0/0"]
}
