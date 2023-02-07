output "endpoint" {
  value = var.os == "mac" ? "${var.name}.macdev.convox.cloud" : "${var.name}.localdev.convox.cloud"
}
