apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${ca}
    server: ${endpoint}
  name: do
contexts:
- context:
    cluster: do
    user: do
  name: do
current-context: do
kind: Config
preferences: {}
users:
- name: do
  user:
    token: ${token}
