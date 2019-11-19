apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentity
metadata:
  namespace: ${namespace}
  name: api
spec:
  type: 0
  ResourceID: ${resource}
  ClientID: ${client}
---
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentityBinding
metadata:
  namespace: ${namespace}
  name: api
spec:
  AzureIdentity: api
  Selector: api