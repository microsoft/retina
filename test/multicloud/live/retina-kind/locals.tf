locals {
  prefix = "mc"

  retina_release_name      = "retina"
  retina_release_namespace = "kube-system"
  retina_repository_url    = "oci://ghcr.io/microsoft/retina/charts"
  retina_chart_version     = "v0.0.24"
  retina_chart_name        = "retina"
  retina_values            = yamldecode(file("../files/retina-standard-advanced-remote-operator.yaml"))

  prometheus_release_name      = "prometheus"
  prometheus_release_namespace = "kube-system"
  prometheus_repository_url    = "https://prometheus-community.github.io/helm-charts"
  prometheus_chart_version     = "68.4.3"
  prometheus_chart_name        = "kube-prometheus-stack"
  prometheus_values            = yamldecode(file("../../../../deploy/standard/prometheus/values.yaml"))
}