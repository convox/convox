resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

resource "google_storage_bucket" "storage" {
  name = "${var.name}-storage-${random_string.suffix.result}"

  force_destroy               = true
  location                    = var.region
  project                     = var.project_id
  uniform_bucket_level_access = true
}
