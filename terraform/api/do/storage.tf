resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

resource "digitalocean_spaces_bucket" "storage" {
  name          = "${var.name}-storage-${random_string.suffix.result}"
  region        = var.region
  acl           = "private"
  force_destroy = true
}
