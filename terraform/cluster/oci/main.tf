data "oci_identity_availability_domains" "rack" {
  compartment_id = var.compartment_ocid
}

data "oci_containerengine_cluster_option" "rack" {
  cluster_option_id = "all"
  compartment_id    = var.compartment_ocid
}

locals {
  # oke reports full versions (v1.35.1) but var.k8s_version is major.minor like the other clouds
  k8s_versions = [for v in data.oci_containerengine_cluster_option.rack.kubernetes_versions : v if length(regexall("^v${var.k8s_version}\\.", v)) > 0]
  k8s_version  = element(sort(local.k8s_versions), length(local.k8s_versions) - 1)
}

resource "oci_containerengine_cluster" "rack" {
  compartment_id     = var.compartment_ocid
  kubernetes_version = local.k8s_version
  name               = var.name
  type               = "BASIC_CLUSTER"
  vcn_id             = oci_core_vcn.rack.id

  endpoint_config {
    is_public_ip_enabled = true
    subnet_id            = oci_core_subnet.endpoint.id
  }

  options {
    service_lb_subnet_ids = [oci_core_subnet.lb.id]
  }
}

data "oci_containerengine_node_pool_option" "rack" {
  compartment_id      = var.compartment_ocid
  node_pool_option_id = oci_containerengine_cluster.rack.id
}

locals {
  # oke image names look like Oracle-Linux-8.10-2025.01.31-0-OKE-1.35.1-754, so the
  # highest sorting name is the newest. "GPU" variants ship the nvidia drivers,
  # "aarch64" ones are arm.
  node_images      = { for s in data.oci_containerengine_node_pool_option.rack.sources : s.source_name => s.image_id if s.source_type == "IMAGE" }
  node_image_names = [for n in keys(local.node_images) : n if length(regexall("GPU|aarch64", n)) == 0]
  gpu_image_names  = [for n in keys(local.node_images) : n if length(regexall("GPU", n)) > 0]
  node_image_id    = local.node_images[element(sort(local.node_image_names), length(local.node_image_names) - 1)]
  gpu_image_id     = try(local.node_images[element(sort(local.gpu_image_names), length(local.gpu_image_names) - 1)], "")
}

resource "oci_containerengine_node_pool" "rack" {
  cluster_id         = oci_containerengine_cluster.rack.id
  compartment_id     = var.compartment_ocid
  kubernetes_version = local.k8s_version
  name               = "${var.name}-node"
  node_shape         = var.node_type

  # oke node pools are fixed size; scaling them needs the cluster-autoscaler add-on
  node_config_details {
    size = var.node_count

    dynamic "placement_configs" {
      for_each = data.oci_identity_availability_domains.rack.availability_domains
      content {
        availability_domain = placement_configs.value.name
        subnet_id           = oci_core_subnet.workers.id
      }
    }
  }

  dynamic "node_shape_config" {
    for_each = length(regexall("Flex", var.node_type)) > 0 ? [1] : []
    content {
      memory_in_gbs = var.node_memory
      ocpus         = var.node_ocpus
    }
  }

  node_source_details {
    boot_volume_size_in_gbs = var.node_disk
    image_id                = local.node_image_id
    source_type             = "IMAGE"
  }

  timeouts {
    update = var.terraform_update_timeout
  }
}

# oke only issues exec kubeconfigs (token_version 2.0.0), so the kubernetes and helm
# providers authenticate by shelling out to `oci ce cluster generate-token`. there is
# no data source that returns a bearer token, so the oci cli must be on the machine
# running terraform. the ca and server are read out of the same kubeconfig because
# oci_containerengine_cluster does not export them.
data "oci_containerengine_cluster_kube_config" "rack" {
  cluster_id    = oci_containerengine_cluster.rack.id
  token_version = "2.0.0"
}

locals {
  kube_config = yamldecode(data.oci_containerengine_cluster_kube_config.rack.content)
  kube_ca     = base64decode(local.kube_config["clusters"][0]["cluster"]["certificate-authority-data"])
  kube_host   = local.kube_config["clusters"][0]["cluster"]["server"]
}

resource "local_file" "kubeconfig" {
  depends_on = [oci_containerengine_node_pool.rack]

  filename = pathexpand("~/.kube/config.oci.${var.name}")
  content  = data.oci_containerengine_cluster_kube_config.rack.content
}

provider "helm" {
  kubernetes {
    host                   = local.kube_host
    cluster_ca_certificate = local.kube_ca

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "oci"
      args        = ["ce", "cluster", "generate-token", "--cluster-id", oci_containerengine_cluster.rack.id, "--region", var.region]
    }
  }
}
