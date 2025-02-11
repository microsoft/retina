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

module "prometheus_lb_gke" {
  depends_on = [
    module.gke,
    module.prometheus_gke
  ]
  source = "../../modules/kubernetes-lb"
}

module "gke_firewall" {
  depends_on             = [module.gke]
  source                 = "../../modules/gke-firewall"
  prefix                 = local.prefix
  inbound_firewall_rule  = local.gke_firwall_rules.inbound
  outbound_firewall_rule = local.gke_firwall_rules.outbound
}

module "grafana" {
  depends_on = [module.prometheus_lb_gke]
  source     = "../../modules/grafana"
  prometheus_endpoints = {
    gke = "http://${module.prometheus_lb_gke.ip}:9090"
  }
}