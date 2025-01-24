output "gcloud_get_kubeconfig" {
  value       = "gcloud container clusters get-credentials ${google_container_cluster.gke.name} --region ${google_container_cluster.gke.location} --project ${google_container_cluster.gke.project}"
  description = "Run this command to fetch the kubeconfig for your GKE cluster"
}

output "host" {
  value = "https://${google_container_cluster.gke.endpoint}"
}

output "client_certificate" {
  value = google_container_cluster.gke.master_auth.0.client_certificate
}

output "client_key" {
  value = google_container_cluster.gke.master_auth.0.client_key
}

output "cluster_ca_certificate" {
  value = google_container_cluster.gke.master_auth.0.cluster_ca_certificate
}