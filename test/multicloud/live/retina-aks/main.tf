module "aks" {
  source              = "../../modules/aks"
  location            = local.location
  resource_group_name = local.resource_group_name
  prefix              = local.prefix
  default_node_pool   = local.default_node_pool
}

module "retina_aks" {
  depends_on        = [module.aks]
  source            = "../../modules/helm-release"
  chart_version     = local.retina_chart_version
  release_name      = local.retina_release_name
  release_namespace = local.retina_release_namespace
  repository_url    = local.retina_repository_url
  chart_name        = local.retina_chart_name
  values            = local.retina_values
}

module "prometheus_aks" {
  depends_on    = [module.aks]
  source        = "../../modules/helm-release"
  chart_version = local.prometheus_chart_version
  values = merge(local.prometheus_values, {
    prometheus = {
      prometheusSpec = {
        additionalScrapeConfigs = <<-EOT
                                - job_name: "retina-pods"
                                  kubernetes_sd_configs:
                                    - role: pod
                                  relabel_configs:
                                    - source_labels: [__meta_kubernetes_pod_container_name]
                                      action: keep
                                      regex: retina
                                    - source_labels:
                                        [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
                                      separator: ":"
                                      regex: ([^:]+)(?::\d+)?
                                      target_label: __address__
                                      replacement: $${1}:$${2}
                                      action: replace
                                    - source_labels: [__meta_kubernetes_pod_node_name]
                                      action: replace
                                      target_label: instance
                                  metric_relabel_configs:
                                    - source_labels: [__name__]
                                      action: keep
                                      regex: (.*)
                                - job_name: networkobservability-hubble
                                  kubernetes_sd_configs:
                                    - role: pod
                                  relabel_configs:
                                    - target_label: cluster
                                      replacement: ${local.cluster_name}
                                      action: replace
                                    - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_pod_label_k8s_app]
                                      regex: kube-system;(retina)
                                      action: keep
                                    - source_labels: [__address__]
                                      action: replace
                                      regex: ([^:]+)(?::\d+)?
                                      replacement: $1:9965
                                      target_label: __address__
                                    - source_labels: [__meta_kubernetes_pod_node_name]
                                      target_label: instance
                                      action: replace
                                  metric_relabel_configs:
                                    - source_labels: [__name__]
                                      regex: '|hubble_dns_queries_total|hubble_dns_responses_total|hubble_drop_total|hubble_tcp_flags_total|hubble_flows_processed_total'
                                      action: keep
                            EOT
      }
    }
  })
  release_name      = local.prometheus_release_name
  release_namespace = local.prometheus_release_namespace
  repository_url    = local.prometheus_repository_url
  chart_name        = local.prometheus_chart_name
}

module "grafana" {
  source            = "../../modules/grafana"
  cluster_reference = "aks"
  dashboards        = local.dashboards
  hosted_grafana_id = var.grafana_pdc_hosted_grafana_id
  grafana_region    = var.grafana_pdc_cluster
}

module "grafana_pdc_aks" {
  depends_on                    = [module.prometheus_aks, module.grafana]
  source                        = "../../modules/grafana-pdc-agent"
  grafana_pdc_token             = module.grafana.pdc_network_token
  grafana_pdc_hosted_grafana_id = var.grafana_pdc_hosted_grafana_id
  grafana_pdc_cluster           = var.grafana_pdc_cluster
}