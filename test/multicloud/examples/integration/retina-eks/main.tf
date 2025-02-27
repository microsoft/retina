module "eks" {
  source = "../../../modules/eks"
  prefix = var.prefix
  region = var.region
}

module "retina" {
  depends_on     = [module.eks]
  source         = "../../../modules/helm-release"
  release_name   = var.retina_release_name
  repository_url = var.retina_repository_url
  chart_version  = var.retina_chart_version
  chart_name     = var.retina_chart_name
  values         = var.retina_values
}
