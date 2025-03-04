output "kubeconfig_command" {
  value = module.gke.gcloud_get_kubeconfig
}
