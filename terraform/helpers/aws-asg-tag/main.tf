variable "asg_name" {
  type = string
}

variable "asg_tags" {
  type = map(string)
}

variable "propagate_at_launch" {
  type    = bool
  default = true
}

resource "aws_autoscaling_group_tag" "cluster_additional" {
  for_each = var.asg_tags

  autoscaling_group_name = var.asg_name

  tag {
    key                 = each.key
    value               = each.value
    propagate_at_launch = var.propagate_at_launch
  }
}
