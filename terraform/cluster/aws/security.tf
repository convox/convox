resource "aws_security_group" "cluster" {
  name        = "${var.name}-cluster"
  description = "${var.name} cluster"
  vpc_id      = aws_vpc.nodes.id

  tags = merge(local.tags, {
    Name = "${var.name}-cluster"
  })
}

resource "aws_security_group_rule" "cluster_ingress_control" {
  type                     = "ingress"
  description              = "control ingress"
  security_group_id        = aws_security_group.cluster.id
  source_security_group_id = aws_security_group.nodes.id
  protocol                 = "tcp"
  from_port                = 443
  to_port                  = 443
}

resource "aws_security_group_rule" "cluster_egress_control" {
  type                     = "egress"
  description              = "control egress"
  security_group_id        = aws_security_group.cluster.id
  source_security_group_id = aws_security_group.nodes.id
  protocol                 = "tcp"
  from_port                = 443
  to_port                  = 443
}

resource "aws_security_group_rule" "cluster_egress_traffic" {
  type                     = "egress"
  description              = "traffic egress"
  security_group_id        = aws_security_group.cluster.id
  source_security_group_id = aws_security_group.nodes.id
  protocol                 = "tcp"
  from_port                = 1025
  to_port                  = 65535
}

resource "aws_security_group" "nodes" {
  name        = "${var.name}-nodes"
  description = "${var.name} nodes"
  vpc_id      = aws_vpc.nodes.id

  # ingress {
  #   description = "mtu discovery"
  #   cidr_blocks = ["0.0.0.0/0"]
  #   protocol    = "icmp"
  #   from_port   = 3
  #   to_port     = 4
  # }

  # ingress {
  #   description     = "control ingress"
  #   security_groups = [aws_security_group.cluster.id]
  #   protocol        = "tcp"
  #   from_port       = 443
  #   to_port         = 443
  # }

  # ingress {
  #   description     = "traffic ingress"
  #   security_groups = [aws_security_group.cluster.id]
  #   protocol        = "tcp"
  #   from_port       = 1025
  #   to_port         = 65535
  # }

  # ingress {
  #   description = "internal ingress"
  #   self        = true
  #   protocol    = -1
  #   from_port   = 0
  #   to_port     = 0
  # }

  # egress {
  #   description = "internet egress"
  #   protocol    = "-1"
  #   cidr_blocks = ["0.0.0.0/0"]
  #   from_port   = 0
  #   to_port     = 0
  # }

  tags = merge(local.tags, {
    Name = "${var.name} nodes"
    "kubernetes.io.cluster/${aws_eks_cluster.cluster.id}" : "owned"
  })
}

resource "aws_security_group_rule" "nodes_ingress_mtu" {
  type              = "ingress"
  description       = "mtu discovery"
  security_group_id = aws_security_group.nodes.id
  cidr_blocks       = ["0.0.0.0/0"]
  protocol          = "icmp"
  from_port         = 3
  to_port           = 4
}

resource "aws_security_group_rule" "nodes_ingress_control" {
  type                     = "ingress"
  description              = "control ingress"
  security_group_id        = aws_security_group.nodes.id
  source_security_group_id = aws_security_group.cluster.id
  protocol                 = "tcp"
  from_port                = 443
  to_port                  = 443
}

resource "aws_security_group_rule" "nodes_ingress_traffic" {
  type                     = "ingress"
  description              = "traffic ingress"
  security_group_id        = aws_security_group.nodes.id
  source_security_group_id = aws_security_group.cluster.id
  protocol                 = "tcp"
  from_port                = 1025
  to_port                  = 65535
}

resource "aws_security_group_rule" "nodes_ingress_internal" {
  type                     = "ingress"
  description              = "internal ingress"
  security_group_id        = aws_security_group.nodes.id
  source_security_group_id = aws_security_group.nodes.id
  protocol                 = -1
  from_port                = 0
  to_port                  = 65535
}

resource "aws_security_group_rule" "nodes_egress_internet" {
  type              = "egress"
  description       = "internet egress"
  security_group_id = aws_security_group.nodes.id
  cidr_blocks       = ["0.0.0.0/0"]
  protocol          = -1
  from_port         = 0
  to_port           = 65535
}
