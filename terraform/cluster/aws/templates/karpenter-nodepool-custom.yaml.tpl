apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: ${name}
spec:
%{ if weight != null }
  weight: ${weight}
%{ endif }
  template:
    metadata:
      labels:
        convox.io/nodepool: ${name}
%{ for k, v in labels }
        ${k}: "${v}"
%{ endfor }
    spec:
      expireAfter: ${node_expiry}
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
          values: [${capacity_types}]
        - key: kubernetes.io/arch
          operator: In
          values: [${arch}]
%{ if instance_families != "" }
        - key: karpenter.k8s.aws/instance-family
          operator: In
          values: [${instance_families}]
%{ endif }
%{ if instance_sizes != "" }
        - key: karpenter.k8s.aws/instance-size
          operator: In
          values: [${instance_sizes}]
%{ endif }
  limits:
    cpu: "${cpu_limit}"
    memory: "${memory_limit_gb}Gi"
  disruption:
    consolidationPolicy: ${consolidation_policy}
    consolidateAfter: ${consolidate_after}
    budgets:
      - nodes: "${disruption_budget_nodes}"
