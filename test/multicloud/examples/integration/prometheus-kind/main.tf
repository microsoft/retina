module "kind" {
  source = "../../../modules/kind"
  prefix = var.prefix
}

module "prometheus" {
  depends_on     = [module.kind]
  source         = "../../../modules/helm-release"
  release_name   = var.prometheus_release_name
  repository_url = var.prometheus_repository_url
  chart_version  = var.prometheus_chart_version
  chart_name     = var.prometheus_chart_name
  values         = var.prometheus_values
}
