MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "CONVOX MANAGED USER DATA SCRIPT"

echo "USER PROVIDED USER DATA SCRIPT"
${user_data}

${user_data_script_file}

%{ if kubelet_registry_pull_qps != 5 || kubelet_registry_burst != 10 ~}
--==MYBOUNDARY==
Content-Type: application/node.eks.aws

apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  kubelet:
    config:
      registryPullQPS: ${kubelet_registry_pull_qps}
      registryBurst: ${kubelet_registry_burst}

%{ endif ~}
--==MYBOUNDARY==--