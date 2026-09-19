locals {
  vcn_cidr = "10.0.0.0/16"

  # the service gateway needs the "All <region> Services" cidr, not the object storage only one
  services = [for s in data.oci_core_services.rack.services : s if length(regexall("All .* Services In Oracle Services Network", s.name)) > 0]
}

data "oci_core_services" "rack" {}

resource "oci_core_vcn" "rack" {
  compartment_id = var.compartment_ocid
  cidr_blocks    = [local.vcn_cidr]
  display_name   = var.name
  dns_label      = substr(replace(lower(var.name), "/[^a-z0-9]/", ""), 0, 15)
}

resource "oci_core_internet_gateway" "rack" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-internet"
  vcn_id         = oci_core_vcn.rack.id
}

resource "oci_core_nat_gateway" "rack" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-nat"
  vcn_id         = oci_core_vcn.rack.id
}

resource "oci_core_service_gateway" "rack" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-services"
  vcn_id         = oci_core_vcn.rack.id

  services {
    service_id = local.services[0].id
  }
}

resource "oci_core_route_table" "public" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-public"
  vcn_id         = oci_core_vcn.rack.id

  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_internet_gateway.rack.id
  }
}

resource "oci_core_route_table" "private" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-private"
  vcn_id         = oci_core_vcn.rack.id

  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_nat_gateway.rack.id
  }

  route_rules {
    destination       = local.services[0].cidr_block
    destination_type  = "SERVICE_CIDR_BLOCK"
    network_entity_id = oci_core_service_gateway.rack.id
  }
}

resource "oci_core_security_list" "endpoint" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-endpoint"
  vcn_id         = oci_core_vcn.rack.id

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  # kubectl and the terraform kubernetes provider
  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"

    tcp_options {
      min = 6443
      max = 6443
    }
  }

  # workers to control plane (6443 and 12250)
  ingress_security_rules {
    protocol = "all"
    source   = local.vcn_cidr
  }
}

resource "oci_core_security_list" "workers" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-workers"
  vcn_id         = oci_core_vcn.rack.id

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  # control plane, other workers, and load balancer health checks / nodeports
  ingress_security_rules {
    protocol = "all"
    source   = local.vcn_cidr
  }

  # path mtu discovery
  ingress_security_rules {
    protocol = "1"
    source   = "0.0.0.0/0"

    icmp_options {
      type = 3
      code = 4
    }
  }
}

resource "oci_core_security_list" "lb" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-lb"
  vcn_id         = oci_core_vcn.rack.id

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"

    tcp_options {
      min = 80
      max = 80
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"

    tcp_options {
      min = 443
      max = 443
    }
  }
}

resource "oci_core_subnet" "endpoint" {
  cidr_block        = cidrsubnet(local.vcn_cidr, 12, 0)
  compartment_id    = var.compartment_ocid
  display_name      = "${var.name}-endpoint"
  dns_label         = "endpoint"
  route_table_id    = oci_core_route_table.public.id
  security_list_ids = [oci_core_security_list.endpoint.id]
  vcn_id            = oci_core_vcn.rack.id
}

resource "oci_core_subnet" "workers" {
  cidr_block                 = cidrsubnet(local.vcn_cidr, 8, 1)
  compartment_id             = var.compartment_ocid
  display_name               = "${var.name}-workers"
  dns_label                  = "workers"
  prohibit_public_ip_on_vnic = true
  route_table_id             = oci_core_route_table.private.id
  security_list_ids          = [oci_core_security_list.workers.id]
  vcn_id                     = oci_core_vcn.rack.id
}

resource "oci_core_subnet" "lb" {
  cidr_block        = cidrsubnet(local.vcn_cidr, 8, 2)
  compartment_id    = var.compartment_ocid
  display_name      = "${var.name}-lb"
  dns_label         = "lb"
  route_table_id    = oci_core_route_table.public.id
  security_list_ids = [oci_core_security_list.lb.id]
  vcn_id            = oci_core_vcn.rack.id
}
