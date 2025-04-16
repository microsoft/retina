module "aks" {
  source              = "../../../modules/aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  prefix              = var.prefix
  labels              = var.labels
}

module "retina" {
  depends_on     = [module.aks]
  source         = "../../../modules/helm-release"
  release_name   = var.retina_release_name
  repository_url = var.retina_repository_url
  chart_version  = var.retina_chart_version
  chart_name     = var.retina_chart_name
  values         = var.retina_values
}
