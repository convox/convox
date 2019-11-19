apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${ca}
    server: ${endpoint}
  name: gcloud
contexts:
- context:
    cluster: gcloud
    user: gcloud
  name: gcloud
current-context: gcloud
kind: Config
preferences: {}
users:
- name: gcloud
  user:
    client-certificate-data: ${client_certificate}
    client-key-data: ${client_key}
