module "gke" {
  source       = "../../modules/gke"
  location     = var.location
  prefix       = var.prefix
  project      = var.project
  machine_type = var.machine_type
}

module "retina" {
  depends_on     = [module.gke]
  source         = "../../modules/helm-release"
  release_name   = var.retina_release_name
  repository_url = var.retina_repository_url
  chart_version  = var.retina_chart_version
  chart_name     = var.retina_chart_name
  values         = var.retina_values
}
