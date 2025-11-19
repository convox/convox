MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="//"

--//
Content-Type: application/node.eks.aws

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    apiServerEndpoint: ${api_server_endpoint}
    certificateAuthority: ${api_server_ca}
    name: ${name}
    cidr: ${cidr}
  kubelet:
    config:
      clusterDNS:
      - ${cluster_dns}
    maxPodsExpression: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
    flags:
    - "--node-labels=${node_labels}"
--//

Content-Type: text/x-shellscript

---
#!/bin/bash
# Custom user data script
${user_data}

--//--
