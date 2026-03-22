apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: ${name}
spec:
  role: "${karpenter_node_role_name}"
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${cluster_name}"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${cluster_name}"
  amiSelectorTerms:
    - alias: al2023@latest
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeType: ${karpenter_node_volume_type}
        volumeSize: "${karpenter_effective_disk}Gi"
        encrypted: ${ebs_encrypted}
  metadataOptions:
    httpTokens: ${imds_http_tokens}
    httpPutResponseHopLimit: ${imds_http_hop_limit}
    httpEndpoint: enabled
  tags:
    Name: "${cluster_name}/karpenter/${name}"
    Rack: "${cluster_name}"
%{ for k, v in extra_tags }
    ${k}: "${v}"
%{ endfor }
