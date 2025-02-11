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

module "prometheus_lb_aks" {
  depends_on = [
    module.aks,
    module.prometheus_aks
  ]
  source = "../../modules/kubernetes-lb"
}

module "aks_nsg" {
  depends_on          = [module.aks]
  source              = "../../modules/aks-nsg"
  prefix              = local.prefix
  resource_group_name = local.resource_group_name
  security_rules      = local.aks_security_rules
}

module "grafana" {
  depends_on = [module.prometheus_lb_aks]
  source     = "../../modules/grafana"
  prometheus_endpoints = {
    aks = "http://${module.prometheus_lb_aks.ip}:9090"
  }
}