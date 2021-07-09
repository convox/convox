provider "google" {
  project = module.project.id
  region  = var.region
}

provider "google-beta" {
  project = module.project.id
  region  = var.region
}

provider "kubernetes" {
  cluster_ca_certificate = module.gke_auth.cluster_ca_certificate
  host                   = module.gke_auth.host
  token                  = module.gke_auth.token

  load_config_file = false
}

module "project" {
  source = "./project"
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/gcp"

  providers = {
    google      = google
    google-beta = google-beta
  }

  name                   = var.name
  node_type              = var.node_type
  preemptible            = var.preemptible
  cluster_ca_certificate = module.gke_auth.cluster_ca_certificate
  host                   = module.gke_auth.host
  token                  = module.gke_auth.token
  kubeconfig_raw         = module.gke_auth.kubeconfig_raw
}

module "gke_auth" {
  source               = "terraform-google-modules/kubernetes-engine/google//modules/auth"

  project_id           = module.project.id
  cluster_name         = var.name
  location             = var.region
}

module "rack" {
  source = "../../rack/gcp"

  providers = {
    kubernetes = kubernetes
    google     = google
  }

  cluster       = module.cluster.id
  name          = var.name
  network       = module.cluster.network
  nodes_account = module.cluster.nodes_account
  release       = local.release
  syslog        = var.syslog
  whitelist     = split(",", var.whitelist)
}
