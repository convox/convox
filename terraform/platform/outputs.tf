output "name" {
  value = trimspace(data.local_file.platform.content)
}
