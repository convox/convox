data "oci_identity_regions" "current" {}

locals {
  # ocir hostnames use the three letter region key (iad.ocir.io), not the region name
  region_key = lower([for r in data.oci_identity_regions.current.regions : r.key if r.name == var.region][0])
}

# an ocir docker password is an iam auth token for the rack user; the matching
# username is <namespace>/<username>
resource "oci_identity_auth_token" "api" {
  description = "convox rack ${var.name} registry"
  user_id     = oci_identity_user.api.id
}
