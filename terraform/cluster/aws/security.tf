resource "aws_security_group" "cluster" {
  name        = "${var.name}-cluster"
  description = "${var.name} cluster"
  vpc_id      = aws_vpc.nodes.id

  tags = merge(local.tags, {
    Name = "${var.name}-cluster"
  })
}
