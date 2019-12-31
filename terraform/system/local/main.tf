terraform {
  required_version = ">= 0.12.0"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10"
}

locals {
  platform_filename = "/tmp/convox.platform"
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases"
}

locals {
  current = jsondecode(data.http.releases.body).0.tag_name
  release = coalesce(var.release, local.current)
}

resource "null_resource" "platform" {
  provisioner "local-exec" {
    command = "uname -s > ${local.platform_filename}"
  }
}

data "local_file" "platform" {
  depends_on = [null_resource.platform]

  filename = local.platform_filename
}

module "rack" {
  source = "../../rack/local"

  providers = {
    kubernetes = kubernetes
  }

  name     = var.name
  platform = trimspace(data.local_file.platform.content)
  release  = local.release
}
