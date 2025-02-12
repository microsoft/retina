module "kind" {
  source = "../../modules/kind"
  prefix = local.prefix
}

module "retina_kind" {
  depends_on        = [module.kind]
  source            = "../../modules/helm-release"
  chart_version     = local.retina_chart_version
  release_name      = local.retina_release_name
  release_namespace = local.retina_release_namespace
  repository_url    = local.retina_repository_url
  chart_name        = local.retina_chart_name
  values            = local.retina_values
}

module "prometheus_kind" {
  depends_on        = [module.kind]
  source            = "../../modules/helm-release"
  chart_version     = local.prometheus_chart_version
  values            = local.prometheus_values
  release_name      = local.prometheus_release_name
  release_namespace = local.prometheus_release_namespace
  repository_url    = local.prometheus_repository_url
  chart_name        = local.prometheus_chart_name
}
