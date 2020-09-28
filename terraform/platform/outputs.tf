output "name" {
  value = trimspace(data.external.platform.result.platform)
}
