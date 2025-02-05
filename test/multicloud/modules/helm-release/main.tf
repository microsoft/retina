resource "helm_release" "release" {
  name       = var.release_name
  repository = var.repository_url
  chart      = var.chart_name
  version    = var.chart_version

  dynamic "set" {
    for_each = var.values
    content {
      name  = set.value.name
      value = set.value.value
    }
  }
}