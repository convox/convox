# output "endpoint" {
#   value = module.router.endpoint
# }

output "endpoint" {
  value = aws_alb.router.dns_name
}
