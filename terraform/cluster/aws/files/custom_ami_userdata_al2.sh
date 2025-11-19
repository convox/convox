#!/bin/bash
set -ex

/etc/eks/bootstrap.sh ${name} \
  --kubelet-extra-args '--node-labels=eks.amazonaws.com/nodegroup=${var.name}-additional-${each.key}-${random_id.additional_node_groups[each.key].hex}' \
  --b64-cluster-ca ${api_server_ca} \
  --apiserver-endpoint ${api_server_endpoint} --use-max-pods true --dns-cluster-ip ${cluster_dns}

# Custom user data script
${user_data}