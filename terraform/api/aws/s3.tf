resource "aws_s3_bucket" "storage" {
  acl           = "private"
  bucket_prefix = "${var.name}-storage-"
  force_destroy = true

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "aws:kms"
      }
    }
  }
}
