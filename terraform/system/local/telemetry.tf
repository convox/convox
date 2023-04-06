locals {
  params            = file("${var.settings}/vars.json")
  send_telemetry    = jsondecode(local.params).telemetry == "false" ? false : true
  telemetry_config  = {
    url              = "https://telemetry.convox.com/telemetry"
    method           = "POST"
    request_headers  = { Accept = "application/json" }
    request_body     = local.params
  }
}

resource "null_resource" "telemetry" {
  count = local.send_telemetry ? 1 : 0

  provisioner "local-exec" {
    command = <<EOF
      curl -X ${local.telemetry_config.method} \
           -H 'Content-Type: application/json' \
           -H 'Accept: application/json' \
           -d '${local.telemetry_config.request_body}' \
           ${local.telemetry_config.url}
    EOF
  }
}