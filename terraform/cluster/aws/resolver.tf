resource "aws_lb_target_group" "resolver" {
  name     = "${var.name}-resolver"
  port     = 31553
  protocol = "UDP"
  vpc_id   = aws_vpc.nodes.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 10
    port                = 31552
    protocol            = "TCP"
    unhealthy_threshold = 2
  }
}

resource "aws_autoscaling_attachment" "resolver_attachment" {
  count = 3

  autoscaling_group_name = aws_eks_node_group.cluster[count.index].resources[0].autoscaling_groups[0].name
  alb_target_group_arn   = aws_lb_target_group.resolver.arn
}

resource "aws_security_group_rule" "resolver-health" {
  cidr_blocks       = ["0.0.0.0/0"]
  from_port         = 31552
  protocol          = "tcp"
  to_port           = 31552
  security_group_id = aws_eks_cluster.cluster.vpc_config.0.cluster_security_group_id
  type              = "ingress"
}


resource "aws_security_group_rule" "resolver-dns" {
  cidr_blocks       = ["0.0.0.0/0"]
  from_port         = 31553
  protocol          = "udp"
  to_port           = 31553
  security_group_id = aws_eks_cluster.cluster.vpc_config.0.cluster_security_group_id
  type              = "ingress"
}
