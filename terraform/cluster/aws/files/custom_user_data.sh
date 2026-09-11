MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "CONVOX MANAGED USER DATA SCRIPT"

echo "USER PROVIDED USER DATA SCRIPT"
${user_data}

${user_data_script_file}

%{ if kubelet_registry_set || fast_image_pull ~}
--==MYBOUNDARY==
Content-Type: application/node.eks.aws

apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
%{ if fast_image_pull ~}
  featureGates:
    FastImagePull: true
%{ endif ~}
%{ if kubelet_registry_set ~}
  kubelet:
    config:
      registryPullQPS: ${kubelet_registry_pull_qps}
      registryBurst: ${kubelet_registry_burst}
%{ endif ~}

%{ endif ~}
--==MYBOUNDARY==--