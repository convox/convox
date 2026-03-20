data "aws_ec2_instance_type" "this" {
  instance_type = var.node_type
}

locals {
  arm_type = contains(data.aws_ec2_instance_type.this.supported_architectures, "arm64")
  gpu_type = substr(var.node_type, 0, 1) == "g" || substr(var.node_type, 0, 1) == "p" || substr(var.node_type, 0, 3) == "inf" || substr(var.node_type, 0, 3) == "trn"
}

variable "node_type" {
  type = string
}

output "ami_type" {
  value = local.gpu_type ? "AL2023_x86_64_NVIDIA" : local.arm_type ? "AL2023_ARM_64_STANDARD" : "AL2023_x86_64_STANDARD"
}

output "is_arm" {
  value = local.arm_type
}
