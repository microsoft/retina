resource "grafana_data_source" "prometheus" {
  name = var.cluster_reference
  type = "prometheus"
  url  = "http://prometheus-operated.kube-system.svc.cluster.local:9090"
  json_data_encoded = jsonencode(
    {
      enableSecureSocksProxy   = true
      httpMethod               = "POST"
      secureSocksProxyUsername = "e1152ac0-72e6-44bf-b7d5-75927827c175"
    }
  )
}

resource "grafana_dashboard" "dashboard" {
  for_each = var.dashboards

  # hardcoded for simplicity, but we might want to make this
  # configurable in the future
  config_json = file("../../../../deploy/grafana-dashboards/${each.value}")
}