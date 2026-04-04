# Karpenter SQS interruption queue and EventBridge rules
# All resources gated on var.karpenter_enabled

###############################################################################
# SQS Queue for Karpenter interruption handling
###############################################################################

resource "aws_sqs_queue" "karpenter_interruption" {
  count = var.karpenter_enabled ? 1 : 0

  name                      = "${var.name}-karpenter"
  message_retention_seconds = 300
  sqs_managed_sse_enabled   = true
  tags                      = local.tags
}

data "aws_iam_policy_document" "karpenter_interruption_queue" {
  count = var.karpenter_enabled ? 1 : 0

  statement {
    sid    = "AllowEventBridgeSend"
    effect = "Allow"
    actions = [
      "sqs:SendMessage",
    ]
    resources = [aws_sqs_queue.karpenter_interruption[0].arn]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com", "sqs.amazonaws.com"]
    }
  }
}

resource "aws_sqs_queue_policy" "karpenter_interruption" {
  count = var.karpenter_enabled ? 1 : 0

  queue_url = aws_sqs_queue.karpenter_interruption[0].id
  policy    = data.aws_iam_policy_document.karpenter_interruption_queue[0].json
}

###############################################################################
# EventBridge Rules — route interruption events to the SQS queue
###############################################################################

resource "aws_cloudwatch_event_rule" "karpenter_scheduled_change" {
  count = var.karpenter_enabled ? 1 : 0

  name          = "${var.name}-karpenter-scheduled-change"
  description   = "Karpenter - AWS Health scheduled change events"
  tags          = local.tags
  event_pattern = jsonencode({
    source      = ["aws.health"]
    detail-type = ["AWS Health Event"]
  })
}

resource "aws_cloudwatch_event_target" "karpenter_scheduled_change" {
  count = var.karpenter_enabled ? 1 : 0

  rule      = aws_cloudwatch_event_rule.karpenter_scheduled_change[0].name
  target_id = "KarpenterInterruptionQueueTarget"
  arn       = aws_sqs_queue.karpenter_interruption[0].arn
}

resource "aws_cloudwatch_event_rule" "karpenter_spot_interruption" {
  count = var.karpenter_enabled ? 1 : 0

  name          = "${var.name}-karpenter-spot-interruption"
  description   = "Karpenter - EC2 Spot Instance interruption warnings"
  tags          = local.tags
  event_pattern = jsonencode({
    source      = ["aws.ec2"]
    detail-type = ["EC2 Spot Instance Interruption Warning"]
  })
}

resource "aws_cloudwatch_event_target" "karpenter_spot_interruption" {
  count = var.karpenter_enabled ? 1 : 0

  rule      = aws_cloudwatch_event_rule.karpenter_spot_interruption[0].name
  target_id = "KarpenterInterruptionQueueTarget"
  arn       = aws_sqs_queue.karpenter_interruption[0].arn
}

resource "aws_cloudwatch_event_rule" "karpenter_rebalance" {
  count = var.karpenter_enabled ? 1 : 0

  name          = "${var.name}-karpenter-rebalance"
  description   = "Karpenter - EC2 Instance rebalance recommendations"
  tags          = local.tags
  event_pattern = jsonencode({
    source      = ["aws.ec2"]
    detail-type = ["EC2 Instance Rebalance Recommendation"]
  })
}

resource "aws_cloudwatch_event_target" "karpenter_rebalance" {
  count = var.karpenter_enabled ? 1 : 0

  rule      = aws_cloudwatch_event_rule.karpenter_rebalance[0].name
  target_id = "KarpenterInterruptionQueueTarget"
  arn       = aws_sqs_queue.karpenter_interruption[0].arn
}

resource "aws_cloudwatch_event_rule" "karpenter_instance_state_change" {
  count = var.karpenter_enabled ? 1 : 0

  name          = "${var.name}-karpenter-instance-state-change"
  description   = "Karpenter - EC2 Instance state change notifications"
  tags          = local.tags
  event_pattern = jsonencode({
    source      = ["aws.ec2"]
    detail-type = ["EC2 Instance State-change Notification"]
  })
}

resource "aws_cloudwatch_event_target" "karpenter_instance_state_change" {
  count = var.karpenter_enabled ? 1 : 0

  rule      = aws_cloudwatch_event_rule.karpenter_instance_state_change[0].name
  target_id = "KarpenterInterruptionQueueTarget"
  arn       = aws_sqs_queue.karpenter_interruption[0].arn
}
