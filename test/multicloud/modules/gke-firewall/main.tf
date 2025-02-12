data "google_compute_network" "vpc_network" {
  name = "${var.prefix}-vpc-network"
}

resource "google_compute_firewall" "gke_inbound_rule" {
  name    = "${var.prefix}-gke-inbound"
  network = data.google_compute_network.vpc_network.id

  allow {
    protocol = var.inbound_firewall_rule.protocol
    ports    = var.inbound_firewall_rule.ports
  }

  source_ranges      = var.inbound_firewall_rule.source_ranges
  destination_ranges = var.inbound_firewall_rule.destination_ranges
  target_tags        = ["${var.prefix}-gke-cluster"]
}

resource "google_compute_firewall" "gke_outbound_rule" {
  name    = "${var.prefix}-gke-outbound"
  network = data.google_compute_network.vpc_network.id

  allow {
    protocol = var.outbound_firewall_rule.protocol
    ports    = var.outbound_firewall_rule.ports
  }

  source_ranges      = var.outbound_firewall_rule.source_ranges
  destination_ranges = var.outbound_firewall_rule.destination_ranges
  target_tags        = ["${var.prefix}-gke-cluster"]
}