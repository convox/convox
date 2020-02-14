resource "aws_dynamodb_table" "cache" {
  name         = "${var.name}-cache"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "key"

  attribute {
    name = "key"
    type = "S"
  }

  tags = local.tags
}

resource "aws_dynamodb_table" "hosts" {
  name         = "${var.name}-hosts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "host"

  attribute {
    name = "host"
    type = "S"
  }

  tags = local.tags
}

resource "aws_dynamodb_table" "routes" {
  name         = "${var.name}-routes"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "host"
  range_key    = "target"

  attribute {
    name = "host"
    type = "S"
  }

  attribute {
    name = "target"
    type = "S"
  }

  tags = local.tags
}
