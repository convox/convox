data "oci_objectstorage_namespace" "current" {
  compartment_id = var.compartment_ocid
}

resource "oci_objectstorage_bucket" "storage" {
  access_type    = "NoPublicAccess"
  compartment_id = var.compartment_ocid
  name           = "${var.name}-storage-${random_string.suffix.result}"
  namespace      = data.oci_objectstorage_namespace.current.namespace
}

# object storage is reached over its s3 compatibility api, so the api needs an
# access key / secret pair rather than an oci api key
resource "oci_identity_customer_secret_key" "api" {
  display_name = "${var.name}-api"
  user_id      = oci_identity_user.api.id
}
