output "endpoint" {
  value = data.http.alias.response_body
}

output "endpoint_internal" {
  value = var.internal_router ? data.http.alias-internal[0].response_body : ""
}
