provider "local" {
  version = "~> 1.4"
}

provider "null" {
  version = "~> 2.1"
}

locals {
  filename = pathexpand("/tmp/convox.platform")
}

resource "null_resource" "platform" {
  triggers = {
    hash = fileexists(local.filename) ? filebase64(local.filename) : "none"
  }

  provisioner "local-exec" {
    command = "mkdir -p ${dirname(local.filename)} && uname -s > ${local.filename}"
  }
}

data "local_file" "platform" {
  depends_on = [null_resource.platform]

  filename = local.filename
}
