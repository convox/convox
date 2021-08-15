module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  image     = var.image
  namespace = var.namespace
  rack      = var.rack
  release   = var.release

}
