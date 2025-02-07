resource "helm_release" "release" {
  name       = var.release_name
  repository = var.repository_url
  chart      = var.chart_name
  version    = var.chart_version
  values     = [jsonencode(var.values)]
}