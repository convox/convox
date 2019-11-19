provider "kubernetes" {
  version = "~> 1.8"

  config_path = var.kubeconfig
}

resource "null_resource" "deployment" {
  provisioner "local-exec" {
    when    = "create"
    command = "kubectl apply -f ${path.module}/deployment.yml"
    environment = {
      "KUBECONFIG" : var.kubeconfig,
    }
  }

  provisioner "local-exec" {
    when    = "destroy"
    command = "kubectl delete -f ${path.module}/deployment.yml"
    environment = {
      "KUBECONFIG" : var.kubeconfig,
    }
  }

  triggers = {
    template = filesha256("${path.module}/deployment.yml")
  }
}
