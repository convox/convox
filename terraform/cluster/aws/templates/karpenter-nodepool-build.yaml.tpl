apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: build
spec:
  template:
    metadata:
      labels:
        convox-build: "true"
        convox.io/nodepool: build
%{ for k, v in extra_labels }
        ${k}: "${v}"
%{ endfor }
    spec:
      expireAfter: ${karpenter_node_expiry}
      taints:
        - key: dedicated
          value: build
          effect: NoSchedule
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: build
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: [${karpenter_build_capacity_types}]
        - key: kubernetes.io/arch
          operator: In
          values: [amd64]
%{ if karpenter_build_instance_families != "" }
        - key: karpenter.k8s.aws/instance-family
          operator: In
          values: [${karpenter_build_instance_families}]
%{ endif }
%{ if karpenter_build_instance_sizes != "" }
        - key: karpenter.k8s.aws/instance-size
          operator: In
          values: [${karpenter_build_instance_sizes}]
%{ endif }
  limits:
    cpu: "${karpenter_build_cpu_limit}"
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: ${karpenter_build_consolidate_after}
