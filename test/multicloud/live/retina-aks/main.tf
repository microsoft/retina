module "aks" {
  source              = "../../modules/aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  prefix              = var.prefix
  labels              = var.labels
}

module "retina" {
  depends_on = [module.aks]
  source     = "../../modules/retina"
}

output "kubeconfig_command" {
  value = module.aks.azure_get_kubeconfig
}
