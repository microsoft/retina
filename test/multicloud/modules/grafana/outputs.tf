output "pdc_network_token" {
  value     = grafana_cloud_private_data_source_connect_network_token.pdc_network_token.token
  sensitive = true
}