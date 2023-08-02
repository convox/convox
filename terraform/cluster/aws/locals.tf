locals {
  validate_private_subnets_count = length(var.private_subnets_ids) == 0 ? true : (var.high_availability ? (length(var.private_subnets_ids) == 3 ? true : tobool("If high availability is enabled, there must be 3 private subnets on each AZ")) : true)
}
