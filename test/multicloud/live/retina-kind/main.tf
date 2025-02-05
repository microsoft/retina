module "kind" {
  source = "../../modules/kind"
  prefix = var.prefix
}

module "retina" {
  depends_on     = [module.kind]
  source         = "../../modules/helm-release"
  release_name   = var.retina_release_name
  repository_url = var.retina_repository_url
  chart_version  = var.retina_chart_version
  chart_name     = var.retina_chart_name
  values         = var.retina_values
}
