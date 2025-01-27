module "aks" {
  source              = "../../modules/aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  prefix              = var.prefix
  labels              = var.labels
}

output kubeconfig {
  value       = module.aks.azure_get_kubeconfig
}
