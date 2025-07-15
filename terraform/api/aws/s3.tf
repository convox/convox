resource "aws_s3_bucket" "storage" { # skipcq: TF-AWS017, TF-AWS002, TF-AWS077
  bucket_prefix = "${var.name}-storage-"
  force_destroy = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "storage" {
  bucket = aws_s3_bucket.storage.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "aws:kms"
    }
  }
}

resource "aws_s3_bucket_ownership_controls" "storage" {
  bucket = aws_s3_bucket.storage.bucket
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_acl" "storage" {
  depends_on = [
    aws_s3_bucket_ownership_controls.storage
  ]

  bucket = aws_s3_bucket.storage.bucket
  acl    = "private"
}


data "aws_s3_bucket" "custom_bucket" {
  count = var.custom_provided_bucket != "" ? 1 : 0
  bucket = var.custom_provided_bucket
}
