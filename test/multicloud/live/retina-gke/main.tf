module "gke" {
  source       = "../../modules/gke"
  location     = local.location
  prefix       = local.prefix
  project      = var.project
  machine_type = local.machine_type
}

module "retina_gke" {
  depends_on        = [module.gke]
  source            = "../../modules/helm-release"
  chart_version     = local.retina_chart_version
  release_name      = local.retina_release_name
  release_namespace = local.retina_release_namespace
  repository_url    = local.retina_repository_url
  chart_name        = local.retina_chart_name
  values            = local.retina_values
}

module "prometheus_gke" {
  depends_on        = [module.gke]
  source            = "../../modules/helm-release"
  chart_version     = local.prometheus_chart_version
  values            = local.prometheus_values
  release_name      = local.prometheus_release_name
  release_namespace = local.prometheus_release_namespace
  repository_url    = local.prometheus_repository_url
  chart_name        = local.prometheus_chart_name
}

module "grafana" {
  source            = "../../modules/grafana"
  cluster_reference = "gke"
  dashboards        = local.grafana_dashboards
  hosted_grafana_id = var.grafana_pdc_hosted_grafana_id
  grafana_region    = var.grafana_pdc_cluster
}

module "grafana_pdc_gke" {
  depends_on                    = [module.prometheus_gke, module.grafana]
  source                        = "../../modules/grafana-pdc-agent"
  grafana_pdc_token             = module.grafana.pdc_network_token
  grafana_pdc_hosted_grafana_id = var.grafana_pdc_hosted_grafana_id
  grafana_pdc_cluster           = var.grafana_pdc_cluster
}
