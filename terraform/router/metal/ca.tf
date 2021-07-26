resource "tls_private_key" "ca-private" {
  algorithm = "RSA"
}

resource "tls_self_signed_cert" "ca" {
  key_algorithm   = tls_private_key.ca-private.algorithm
  private_key_pem = tls_private_key.ca-private.private_key_pem

  dns_names             = ["ca.${var.name}"]
  is_ca_certificate     = true
  set_subject_key_id    = true
  validity_period_hours = 24 * 365 * 10

  allowed_uses = [
    "cert_signing",
    "digital_signature",
    "key_encipherment",
    "server_auth"
  ]

  subject {
    common_name  = "ca.${var.name}"
    organization = "Convox"
  }
}

resource "kubernetes_secret" "ca" {
  metadata {
    namespace = var.namespace
    name      = "ca"
  }

  type = "kubernetes.io/tls"

  data = {
    "tls.crt" = tls_self_signed_cert.ca.cert_pem,
    "tls.key" = tls_private_key.ca-private.private_key_pem,
  }
}
