module "aks" {
  source              = "../../modules/aks"
  location            = local.location
  resource_group_name = local.resource_group_name
  prefix              = local.prefix
  default_node_pool   = local.default_node_pool
}

module "retina_aks" {
  depends_on        = [module.aks]
  source            = "../../modules/helm-release"
  chart_version     = local.retina_chart_version
  release_name      = local.retina_release_name
  release_namespace = local.retina_release_namespace
  repository_url    = local.retina_repository_url
  chart_name        = local.retina_chart_name
  values            = local.retina_values
}

module "prometheus_aks" {
  depends_on        = [module.aks]
  source            = "../../modules/helm-release"
  chart_version     = local.prometheus_chart_version
  values            = local.prometheus_values
  release_name      = local.prometheus_release_name
  release_namespace = local.prometheus_release_namespace
  repository_url    = local.prometheus_repository_url
  chart_name        = local.prometheus_chart_name
}

module "grafana_pdc_aks" {
  depends_on                    = [module.prometheus_aks]
  source                        = "../../modules/grafana-pdc-agent"
  grafana_pdc_token             = var.grafana_pdc_token
  grafana_pdc_hosted_grafana_id = var.grafana_pdc_hosted_grafana_id
  grafana_pdc_cluster           = var.grafana_pdc_cluster
}

module "grafana" {
  source            = "../../modules/grafana"
  cluster_reference = "aks"
  # All dashboards are deployed as part of live/retina-eks
  dashboards = local.dashboards
}
