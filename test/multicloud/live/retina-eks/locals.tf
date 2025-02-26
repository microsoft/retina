locals {
  prefix = "mc"
  region = "eu-west-1"

  retina_release_name      = "retina"
  retina_repository_url    = "oci://ghcr.io/microsoft/retina/charts"
  retina_chart_version     = "v0.0.24"
  retina_release_namespace = "kube-system"
  retina_chart_name        = "retina-hubble"
  retina_values            = yamldecode(file("../files/retina-hubble.yaml"))

  prometheus_release_name      = "prometheus"
  prometheus_repository_url    = "https://prometheus-community.github.io/helm-charts"
  prometheus_chart_version     = "68.4.3"
  prometheus_chart_name        = "kube-prometheus-stack"
  prometheus_release_namespace = "kube-system"
  prometheus_values            = yamldecode(file("../../../../deploy/hubble/prometheus/values.yaml"))

  dashboards = {
    "clusters"                   = "clusters.json"
    "hubble-dns"                 = "hubble-dns.json"
    "hubble-pod-flows-namespace" = "hubble-pod-flows-namespace.json"
    "hubble-pod-flows-workload"  = "hubble-pod-flows-workload.json"
    "standard-dns"               = "standard-dns.json"
    "standard-pod-level"         = "standard-pod-level.json"
  }
}