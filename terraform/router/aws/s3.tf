resource "aws_kms_key" "logs-key" {
  count = var.lb_access_log_enable ? 1 : 0
  description = "rack ${var.name} nlb logs bucket encryption key"
}

resource "aws_s3_bucket" "logs" { # skipcq: TF-AWS017, TF-AWS002, TF-AWS077
  count = var.lb_access_log_enable ? 1 : 0
  bucket_prefix = "${var.name}-nlb-logs-"
  force_destroy = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "logs" {
  count = var.lb_access_log_enable ? 1 : 0
  bucket = aws_s3_bucket.logs[0].bucket

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.logs-key[0].key_id
      sse_algorithm = "aws:kms"
    }
  }
}

resource "aws_s3_bucket_ownership_controls" "logs" {
  count = var.lb_access_log_enable ? 1 : 0
  bucket = aws_s3_bucket.logs[0].bucket
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_acl" "logs" {
  count = var.lb_access_log_enable ? 1 : 0

  depends_on = [
    aws_s3_bucket_ownership_controls.logs
  ]

  bucket = aws_s3_bucket.logs[0].bucket
  acl    = "private"
}

resource "aws_s3_bucket_policy" "allow_access_for_logs" {
  count = var.lb_access_log_enable ? 1 : 0
  bucket = aws_s3_bucket.logs[0].bucket
  policy = data.aws_iam_policy_document.allow_access_for_logs[0].json
}

data "aws_iam_policy_document" "allow_access_for_logs" {
  count = var.lb_access_log_enable ? 1 : 0
  statement {
    principals {
      type        = "Service"
      identifiers = ["delivery.logs.amazonaws.com"]
    }

    actions = [
      "s3:GetBucketAcl",
      "s3:PutObject",
    ]

    resources = [
      aws_s3_bucket.logs[0].arn,
      "${aws_s3_bucket.logs[0].arn}/*",
    ]
  }
}

resource "aws_iam_role" "logs" {
  count = var.lb_access_log_enable ? 1 : 0

  name = "${var.name}-iam-role-for-grant-logs"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "delivery.logs.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_kms_grant" "logs" {
  count = var.lb_access_log_enable ? 1 : 0
  name              = "logs-grant"
  key_id            = aws_kms_key.logs-key[0].key_id
  grantee_principal = aws_iam_role.logs[0].arn
  operations        = ["Encrypt", "Decrypt", "GenerateDataKey", 
  "GenerateDataKeyWithoutPlaintext", "ReEncryptFrom", "ReEncryptTo", "DescribeKey",
  "GenerateDataKeyPair", "GenerateDataKeyPairWithoutPlaintext"]
}
