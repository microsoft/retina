resource "google_service_account" "default" {
  account_id   = "${var.prefix}-gke-service-account"
  display_name = "GKE Service Account for ${var.project}"
}

// Create VPC network
resource "google_compute_network" "vpc_network" {
  name                    = "${var.prefix}-vpc-network"
  auto_create_subnetworks = false
}

// Create subnet within the VPC network
resource "google_compute_subnetwork" "subnet" {
  name          = "${var.prefix}-subnet"
  ip_cidr_range = var.subnet_cidr
  region        = var.location
  network       = google_compute_network.vpc_network.id
}

// https://cloud.google.com/kubernetes-engine/docs/concepts/network-overview
resource "google_container_cluster" "gke" {
  name     = "${var.prefix}-gke-cluster"
  location = var.location

  # We can't create a cluster with no node pool defined, but we want to only use
  # separately managed node pools. So we create the smallest possible default
  # node pool and immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false

  network    = google_compute_network.vpc_network.id
  subnetwork = google_compute_subnetwork.subnet.id
}

resource "google_container_node_pool" "gke_preemptible_nodes" {
  name       = "${var.prefix}-node-pool"
  location   = var.location
  cluster    = google_container_cluster.gke.name
  node_count = 1

  node_config {
    preemptible  = true
    machine_type = var.machine_type

    # Google recommends custom service accounts that have cloud-platform scope and permissions granted via IAM Roles.
    service_account = google_service_account.default.email
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}
