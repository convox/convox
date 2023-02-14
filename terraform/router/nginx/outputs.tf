output "selector" {
  value = {
    system  = "convox"
    service = "ingress-nginx"
  }
}

output "selector-internal" {
  value = {
    system  = "convox"
    service = "ingress-nginx-internal"
  }
}
