module "eks" {
  source = "../../modules/eks"
  region = local.region
  prefix = local.prefix
}

module "retina_eks" {
  depends_on        = [module.eks]
  source            = "../../modules/helm-release"
  chart_version     = local.retina_chart_version
  release_name      = local.retina_release_name
  release_namespace = local.retina_release_namespace
  repository_url    = local.retina_repository_url
  chart_name        = local.retina_chart_name
  values            = local.retina_values
}

module "prometheus_eks" {
  depends_on        = [module.eks]
  source            = "../../modules/helm-release"
  chart_version     = local.prometheus_chart_version
  values            = local.prometheus_values
  release_name      = local.prometheus_release_name
  release_namespace = local.prometheus_release_namespace
  repository_url    = local.prometheus_repository_url
  chart_name        = local.prometheus_chart_name
}

module "prometheus_lb_eks" {
  depends_on = [
    module.eks,
    module.prometheus_eks
  ]
  source = "../../modules/kubernetes-lb"
}

# module "eks_firewall" {
#   depends_on             = [module.eks]
#   source                 = "../../modules/eks-firewall"
#   prefix                 = local.prefix
#   inbound_firewall_rule  = local.gke_firewall_rules.inbound
#   outbound_firewall_rule = local.gke_firewall_rules.outbound
# }

module "grafana" {
  depends_on = [module.prometheus_lb_eks]
  source     = "../../modules/grafana"
  prometheus_endpoints = {
    eks = "http://${module.prometheus_lb_eks.hostname}:9090" # Note: EKS uses hostname instead of IP
  }
  dashboards = local.dashboards
}