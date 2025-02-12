resource "grafana_data_source" "prometheus" {
  for_each = var.prometheus_endpoints

  name = each.key
  type = "prometheus"
  url  = each.value
}