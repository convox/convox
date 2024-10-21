output "endpoint" {
  value = var.convox_rack_domain == "" ? data.http.alias.response_body : var.convox_rack_domain
}

output "endpoint_internal" {
  value = var.internal_router ? data.http.alias-internal[0].response_body : ""
}
