module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  cluster_id = var.cluster_id
  docker_hub_authentication = var.docker_hub_authentication
  image                     = var.image
  namespace                 = var.namespace
  rack                      = var.rack
  release                   = var.release
  replicas                  = var.high_availability ? 2 : 1
}
