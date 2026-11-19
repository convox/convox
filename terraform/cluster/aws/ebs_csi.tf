data "aws_iam_policy_document" "assume_pod_identity" {
  statement {
    actions = ["sts:AssumeRole", "sts:TagSession"]
    effect  = "Allow"

    principals {
      identifiers = ["pods.eks.amazonaws.com"]
      type        = "Service"
    }
  }
}

resource "aws_iam_role" "ebs_csi" {
  count = var.pod_identity_agent_enable ? 1 : 0

  name                  = "${var.name}-ebs-csi"
  assume_role_policy    = data.aws_iam_policy_document.assume_pod_identity.json
  force_detach_policies = true
  path                  = "/convox/"
  tags                  = local.tags
}

resource "aws_iam_role_policy_attachment" "ebs_csi" {
  count = var.pod_identity_agent_enable ? 1 : 0

  role       = aws_iam_role.ebs_csi[0].name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

# The provider does not retry UpdateAddon, and EKS rejects a role IAM has not propagated.
resource "null_resource" "delay_ebs_csi" {
  count = var.pod_identity_agent_enable ? 1 : 0

  provisioner "local-exec" {
    command = "sleep 15"
  }

  triggers = {
    attachment = aws_iam_role_policy_attachment.ebs_csi[0].id
  }
}
