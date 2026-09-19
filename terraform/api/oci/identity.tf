resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

# iam users and groups always live in the tenancy root, not the rack compartment
resource "oci_identity_user" "api" {
  compartment_id = var.tenancy_ocid
  description    = "convox rack ${var.name} api"
  name           = "${var.name}-api-${random_string.suffix.result}"
}

resource "oci_identity_group" "api" {
  compartment_id = var.tenancy_ocid
  description    = "convox rack ${var.name} api"
  name           = "${var.name}-api-${random_string.suffix.result}"
}

resource "oci_identity_user_group_membership" "api" {
  group_id = oci_identity_group.api.id
  user_id  = oci_identity_user.api.id
}

# ocir repositories are created on first push, which needs "manage repos" in the
# tenancy, so nothing pre-creates oci_artifacts_container_repository here
resource "oci_identity_policy" "api" {
  compartment_id = var.tenancy_ocid
  description    = "convox rack ${var.name} api"
  name           = "${var.name}-api-${random_string.suffix.result}"

  statements = [
    "Allow group ${oci_identity_group.api.name} to manage repos in tenancy",
    "Allow group ${oci_identity_group.api.name} to read buckets in compartment id ${var.compartment_ocid}",
    "Allow group ${oci_identity_group.api.name} to manage objects in compartment id ${var.compartment_ocid} where target.bucket.name = '${oci_objectstorage_bucket.storage.name}'",
  ]
}
