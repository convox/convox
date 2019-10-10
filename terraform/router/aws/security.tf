
resource "aws_security_group_rule" "nodes_ingress_router" {
  type              = "ingress"
  description       = "router ingress"
  security_group_id = var.nodes_security
  cidr_blocks       = ["0.0.0.0/0"]
  protocol          = "tcp"
  from_port         = 32000
  to_port           = 32001
}
