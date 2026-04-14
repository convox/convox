variable "cluster" {
  type = string
}

variable "azure_files_enabled" {
  default = false
  type    = bool
}

variable "fluentd_memory" {
  type    = string
  default = "200Mi"
}

variable "docker_hub_authentication" {
  type = string
}

variable "domain" {
  type = string
}

variable "domain_internal" {
  type    = string
  default = ""
}

variable "high_availability" {
  default = true
  type    = bool
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

variable "namespace" {
  type = string
}

variable "pdb_default_min_available_percentage" {
  default = "50"
  type    = string
}

variable "region" {
  type = string
}

variable "release" {
  type = string
}

variable "resolver" {
  type = string
}

variable "resource_group" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "resource_group_location" {
  type = string
}

variable "router" {
  type = string
}

variable "syslog" {
  default = ""
}

variable "workspace" {
  type = string
}
