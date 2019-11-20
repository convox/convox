apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${ca}
    server: ${endpoint}
  name: azure
contexts:
- context:
    cluster: azure
    user: azure
  name: azure
current-context: azure
kind: Config
preferences: {}
users:
- name: azure
  user:
    client-certificate-data: ${client_certificate}
    client-key-data: ${client_key}
