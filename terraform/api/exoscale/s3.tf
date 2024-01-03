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
  name = "${var.name}-sos-admin-role"
  description = "SOS registry bucket admin role"
  editable = true

  policy = {
    default-service-strategy = "deny"
    services = {
      sos = {
        type = "rules"
        rules = [
          {
            expression = "parameters.bucket == ${local.bucket_name}"
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

provider "aws" {

  endpoints {
    s3 = local.s3_region_endpoint
  }

  region     = var.zone

  access_key = exoscale_iam_api_key.api_key.key
  secret_key = exoscale_iam_api_key.api_key.secret

  # Disable AWS-specific features
  skip_credentials_validation = true
  skip_region_validation      = true
  skip_requesting_account_id  = true
  # add this when we update aws terraform provider version
  # skip_s3_checksum            = true
}

resource "aws_s3_bucket" "storage_bucket" {
  bucket   = local.bucket_name
}

resource "aws_s3_bucket_acl" "storage_bucket_acl" {
  bucket = aws_s3_bucket.storage_bucket.id
  acl    = "private"
}
