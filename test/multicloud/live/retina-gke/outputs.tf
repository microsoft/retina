output "kubeconfig_command" {
  value = module.gke.gcloud_get_kubeconfig
}

output "cluster_name" {
  value = module.gke.cluster_name
}