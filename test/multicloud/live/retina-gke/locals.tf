locals {
  prefix   = "mc"
  location = "europe-west2"

  machine_type = "e2-standard-4"

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
  prometheus_values            = yamldecode(file("../../../../deploy/standard/prometheus/values.yaml"))

  gke_firwall_rules = {
    inbound = {
      protocol           = "tcp"
      ports              = ["9090"]
      source_ranges      = [module.prometheus_lb_gke.ip]
      destination_ranges = ["0.0.0.0/0"]
    }
    outbound = {
      protocol           = "tcp"
      ports              = ["9090"]
      source_ranges      = ["0.0.0.0/0"]
      destination_ranges = [module.prometheus_lb_gke.ip]
    }
  }
}