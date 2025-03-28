resource "grafana_cloud_private_data_source_connect_network" "pdc_network" {
  region           = var.grafana_region
  name             = "pdc-${var.cluster_reference}"
  display_name     = "${var.cluster_reference} PDC"
  stack_identifier = var.hosted_grafana_id
}

resource "grafana_cloud_private_data_source_connect_network_token" "pdc_network_token" {
  pdc_network_id = grafana_cloud_private_data_source_connect_network.pdc_network.pdc_network_id
  region         = grafana_cloud_private_data_source_connect_network.pdc_network.region
  name           = "pdc-token-${var.cluster_reference}"
  display_name   = "${var.cluster_reference} PDC Token"
}

resource "grafana_data_source" "prometheus" {
  name                                   = var.cluster_reference
  type                                   = "prometheus"
  url                                    = "http://prometheus-operated.kube-system.svc.cluster.local:9090"
  private_data_source_connect_network_id = grafana_cloud_private_data_source_connect_network.pdc_network.pdc_network_id
  json_data_encoded = jsonencode(
    {
      enableSecureSocksProxy   = true
      secureSocksProxyUsername = grafana_cloud_private_data_source_connect_network.pdc_network.pdc_network_id
    }
  )
}

resource "grafana_dashboard" "dashboard" {
  for_each = var.dashboards

  # hardcoded for simplicity, but we might want to make this
  # configurable in the future
  config_json = file("../../../../deploy/grafana-dashboards/${each.value}")
}