resource "helm_release" "release" {
  name          = var.release_name
  namespace     = var.release_namespace
  repository    = var.repository_url
  chart         = var.chart_name
  version       = var.chart_version
  recreate_pods = true
  values        = [jsonencode(var.values)]

  dynamic "set" {
    for_each = var.custom_values
    content {
      name  = set.key
      value = set.value
    }
  }
}