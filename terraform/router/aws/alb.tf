resource "aws_alb" "router" {
  name               = "${var.name}-router"
  load_balancer_type = "network"
  subnets            = var.subnets
}

resource "aws_alb_listener" "http" {
  load_balancer_arn = aws_alb.router.arn
  port              = 80
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = var.target_group_http
  }
}

resource "aws_alb_listener" "https" {
  load_balancer_arn = aws_alb.router.arn
  port              = 443
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = var.target_group_https
  }
}
