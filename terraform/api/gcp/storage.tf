resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

resource "google_storage_bucket" "storage" {
  name = "${var.name}-storage-${random_string.suffix.result}"

  bucket_policy_only = true
  force_destroy      = true
}
