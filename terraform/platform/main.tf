data "external" "platform" {
  program = ["${path.module}/platform"]
}

data "external" "arch" {
  program = ["${path.module}/arch"]
}
