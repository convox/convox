apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: ${name}
spec:
  template:
    metadata:
      labels:
        convox.io/nodepool: ${name}
%{ for k, v in extra_labels }
        ${k}: "${v}"
%{ endfor }
    spec:
      expireAfter: ${karpenter_node_expiry}
%{ if length(taints) > 0 }
      taints:
%{ for t in taints }
        - key: ${t.key}
          value: "${t.value}"
          effect: ${t.effect}
%{ endfor }
%{ endif }
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: ${name}
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: [${karpenter_capacity_types}]
        - key: kubernetes.io/arch
          operator: In
          values: [${karpenter_arch}]
%{ if karpenter_instance_families != "" }
        - key: karpenter.k8s.aws/instance-family
          operator: In
          values: [${karpenter_instance_families}]
%{ endif }
%{ if karpenter_instance_sizes != "" }
        - key: karpenter.k8s.aws/instance-size
          operator: In
          values: [${karpenter_instance_sizes}]
%{ endif }
  limits:
    cpu: "${karpenter_cpu_limit}"
    memory: "${karpenter_memory_limit_gb}Gi"
  disruption:
    consolidationPolicy: ${karpenter_consolidation_enabled ? "WhenEmptyOrUnderutilized" : "WhenEmpty"}
    consolidateAfter: ${karpenter_consolidate_after}
    budgets:
      - nodes: "${karpenter_disruption_budget_nodes}"
