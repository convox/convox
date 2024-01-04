resource "random_string" "suffix" {
  length  = 10
  special = false
  upper   = false
}

locals {
  bucket_name = "${var.name}-storage-${random_string.suffix.result}"
  s3_region_endpoint = "https://sos-${var.zone}.exo.io"
}

resource "exoscale_iam_role" "api_role" {
  name = "${var.name}-api-role"
  description = "SOS registry bucket admin role"
  editable = true

  policy = {
    default_service_strategy = "deny"
    services = {
      sos = {
        type = "rules"
        rules = [
          {
            expression = "parameters.bucket == '${local.bucket_name}'"
            action = "allow"
          }
        ]
      }

      compute = {
        type = "rules"
        rules = [
          {
            expression = "operation.startsWith('get-') || operation.startsWith('list-')"
            action = "allow"
          }
        ]
      }
    }
  }
}

resource "exoscale_iam_api_key" "api_key" {
  name = "${var.name}-api-key"
  role_id = exoscale_iam_role.api_role.id
}

resource "aws_s3_bucket" "storage_bucket" {
  bucket   = local.bucket_name
  force_destroy = true
}

resource "aws_s3_bucket_acl" "storage_bucket_acl" {
  bucket = aws_s3_bucket.storage_bucket.id
  acl    = "private"
}
