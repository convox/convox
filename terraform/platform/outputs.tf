output "name" {
  value = trimspace(data.external.platform.result.platform)
}

output "arch" {
  value = trimspace(data.external.arch.result.arch)
}
