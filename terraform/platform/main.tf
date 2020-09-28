provider "external" {
  version = "~> 1.2"
}

data "external" "platform" {
  program = ["${path.module}/platform"]
}
