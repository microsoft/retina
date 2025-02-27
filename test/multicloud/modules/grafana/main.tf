resource "grafana_data_source" "prometheus" {
  for_each = var.prometheus_endpoints

  name = each.key
  type = "prometheus"
  url  = each.value
}

resource "grafana_dashboard" "dashboard" {
  for_each = var.dashboards

  # hardcoded for simplicity, but we might want to make this
  # configurable in the future
  config_json = file("../../../../deploy/grafana-dashboards/${each.value}")
}