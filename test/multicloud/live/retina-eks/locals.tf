locals {
  prefix = "mc"
  region = "eu-west-1"

  retina_release_name      = "retina"
  retina_repository_url    = "oci://ghcr.io/microsoft/retina/charts"
  retina_chart_version     = "v0.0.29"
  retina_release_namespace = "kube-system"
  retina_chart_name        = "retina-hubble"
  retina_values            = yamldecode(file("../files/retina-hubble.yaml"))

  prometheus_release_name      = "prometheus"
  prometheus_repository_url    = "https://prometheus-community.github.io/helm-charts"
  prometheus_chart_version     = "68.4.3"
  prometheus_chart_name        = "kube-prometheus-stack"
  prometheus_release_namespace = "kube-system"
  prometheus_values            = yamldecode(file("../../../../deploy/hubble/prometheus/values.yaml"))
  cluster_name                 = "ElasticKubernetesService"

  # All dashboards are deployed as part of live/retina-aks
  dashboards = {}
}