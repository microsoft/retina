resource "helm_release" "prometheus" {
  name       = "prometheus"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  version    = var.prometheus_version

  dynamic "set" {
    for_each = var.values
    content {
      name  = set.value.name
      value = set.value.value
    }
  }
}