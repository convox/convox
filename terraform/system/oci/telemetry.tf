
// this is auto generated(do not edit manually): go run cmd/telemetry-gen/main.go oci

locals {
  telemetry_map = {
    buildkit_enabled         = var.buildkit_enabled
    cert_duration            = var.cert_duration
    compartment_ocid         = var.compartment_ocid
    docker_hub_password      = var.docker_hub_password
    docker_hub_username      = var.docker_hub_username
    fluentd_memory           = var.fluentd_memory
    gpu_node_count           = var.gpu_node_count
    gpu_node_type            = var.gpu_node_type
    high_availability        = var.high_availability
    image                    = var.image
    k8s_version              = var.k8s_version
    name                     = var.name
    node_count               = var.node_count
    node_disk                = var.node_disk
    node_memory              = var.node_memory
    node_ocpus               = var.node_ocpus
    node_type                = var.node_type
    rack_name                = var.rack_name
    region                   = var.region
    release                  = var.release
    settings                 = var.settings
    syslog                   = var.syslog
    telemetry                = var.telemetry
    tenancy_ocid             = var.tenancy_ocid
    terraform_update_timeout = var.terraform_update_timeout
    user_ocid                = var.user_ocid
    webhook_signing_key      = var.webhook_signing_key
    whitelist                = var.whitelist
  }

  telemetry_default_map = {
    buildkit_enabled         = "false"
    cert_duration            = "2160h"
    compartment_ocid         = ""
    docker_hub_password      = ""
    docker_hub_username      = ""
    fluentd_memory           = "200Mi"
    gpu_node_count           = "1"
    gpu_node_type            = ""
    high_availability        = "true"
    image                    = "convox/convox"
    k8s_version              = "1.35"
    name                     = ""
    node_count               = "2"
    node_disk                = "100"
    node_memory              = "16"
    node_ocpus               = "2"
    node_type                = "VM.Standard.E4.Flex"
    rack_name                = ""
    region                   = "us-ashburn-1"
    release                  = ""
    settings                 = ""
    syslog                   = ""
    telemetry                = "false"
    tenancy_ocid             = ""
    terraform_update_timeout = "2h"
    user_ocid                = ""
    webhook_signing_key      = ""
    whitelist                = "0.0.0.0/0"
  }
}
