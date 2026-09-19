locals {
  # oke node pools have no taints argument, so the taint is registered by a custom
  # cloud-init that re-runs the oke bootstrap script with extra kubelet flags.
  gpu_user_data = <<-EOF
    #!/bin/bash
    curl --fail -H "Authorization: Bearer Oracle" -L0 http://169.254.169.254/opc/v2/instance/metadata/oke_init_script | base64 --decode >/var/run/oke-init.sh
    bash /var/run/oke-init.sh --kubelet-extra-args "--register-with-taints=nvidia.com/gpu=:NoSchedule"
  EOF
}

resource "oci_containerengine_node_pool" "gpu" {
  count = var.gpu_node_type != "" ? 1 : 0

  cluster_id         = oci_containerengine_cluster.rack.id
  compartment_id     = var.compartment_ocid
  kubernetes_version = local.k8s_version
  name               = "${var.name}-gpu"
  node_shape         = var.gpu_node_type

  initial_node_labels {
    key   = "convox.io/gpu-vendor"
    value = "nvidia"
  }

  node_config_details {
    size = var.gpu_node_count

    dynamic "placement_configs" {
      for_each = data.oci_identity_availability_domains.rack.availability_domains
      content {
        availability_domain = placement_configs.value.name
        subnet_id           = oci_core_subnet.workers.id
      }
    }
  }

  node_metadata = {
    user_data = base64encode(local.gpu_user_data)
  }

  node_source_details {
    boot_volume_size_in_gbs = var.node_disk
    image_id                = local.gpu_image_id
    source_type             = "IMAGE"
  }

  timeouts {
    update = var.terraform_update_timeout
  }
}
