variable "node_type" {
  type = string
}

output "gpu_type" {
  value = substr(var.node_type, 0, 1) == "g" || substr(var.node_type, 0, 1) == "p" || substr(var.node_type, 0, 3) == "inf" || substr(var.node_type, 0, 3) == "trn"
}

output "arm_type" {
  value = substr(var.node_type, 0, 2) == "a1" || substr(var.node_type, 0, 5) == "hpc7g" || substr(var.node_type, 0, 4) == "im4g" || substr(var.node_type, 0, 4) == "is4g" || substr(var.node_type, 2, 1) == "g"
}