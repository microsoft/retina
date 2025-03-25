output "kubeconfig_command" {
  value = module.aks.azure_get_kubeconfig
}

output "cluster_name" {
  value = module.aks.cluster_name
}
