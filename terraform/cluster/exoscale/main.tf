resource "exoscale_sks_cluster" "rack" {
  name    = var.name
  zone    = var.zone
  version = var.k8s_version
  auto_upgrade = false
  cni = "calico"
  exoscale_ccm = true
  labels = {
    "convox" = "rack"
    "rack"   = var.name
  }
  metrics_server = true
}

# (ad-hoc anti-affinity group)
resource "exoscale_anti_affinity_group" "sks_anti_affinity_group" {
  name = "${var.name}-sks-anti-affinity-group"
}

# (ad-hoc security group)
resource "exoscale_security_group" "sks_security_group" {
  name = "${var.name}-security-group"
}

resource "exoscale_security_group_rule" "kubelet" {
  security_group_id = exoscale_security_group.sks_security_group.id
  description       = "Kubelet"
  type              = "INGRESS"
  protocol          = "TCP"
  start_port        = 10250
  end_port          = 10250
  # (beetwen worker nodes only)
  user_security_group_id = exoscale_security_group.sks_security_group.id
}

resource "exoscale_security_group_rule" "calico_vxlan" {
  security_group_id = exoscale_security_group.sks_security_group.id
  description       = "VXLAN (Calico)"
  type              = "INGRESS"
  protocol          = "UDP"
  start_port        = 4789
  end_port          = 4789
  # (beetwen worker nodes only)
  user_security_group_id = exoscale_security_group.sks_security_group.id
}

resource "exoscale_security_group_rule" "nodeport_tcp" {
  security_group_id = exoscale_security_group.sks_security_group.id
  description       = "Nodeport TCP services"
  type              = "INGRESS"
  protocol          = "TCP"
  start_port        = 30000
  end_port          = 32767
  # (public)
  cidr = "0.0.0.0/0"
}

resource "exoscale_security_group_rule" "nodeport_udp" {
  security_group_id = exoscale_security_group.sks_security_group.id
  description       = "Nodeport UDP services"
  type              = "INGRESS"
  protocol          = "UDP"
  start_port        = 30000
  end_port          = 32767
  # (public)
  cidr = "0.0.0.0/0"
}

# (worker nodes)
resource "exoscale_sks_nodepool" "sks_nodepool" {
  zone       = var.zone
  cluster_id = exoscale_sks_cluster.rack.id
  name       = "${var.name}-nodepool"

  instance_type = var.instance_type
  size          = var.high_availability ? 3 : 1

  disk_size = var.instance_disk_size

  anti_affinity_group_ids = [
    exoscale_anti_affinity_group.sks_anti_affinity_group.id,
  ]
  security_group_ids = [
    exoscale_security_group.sks_security_group.id,
  ]
}

# (administration credentials)
resource "exoscale_sks_kubeconfig" "sks_kubeconfig" {
  zone       = var.zone
  cluster_id = exoscale_sks_cluster.rack.id

  user   = "kubernetes-admin"
  groups = ["system:masters"]

  ttl_seconds           = 30 * 86400
  early_renewal_seconds = 300
}

resource "null_resource" "delay" {
  depends_on = [ 
    exoscale_sks_cluster.rack,
    exoscale_sks_nodepool.sks_nodepool
  ]
  provisioner "local-exec" {
    command = "sleep 30"
  }
}

resource "local_file" "kubeconfig" {
  depends_on = [null_resource.delay]

  filename = pathexpand("~/.kube/config.exoscale.${var.name}")
  content         = exoscale_sks_kubeconfig.sks_kubeconfig.kubeconfig
}
