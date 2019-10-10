output "endpoint" {
  value = "https://convox:${random_string.password.result}@api.${var.domain}"
}
