module "grafana" {
  source = "../../modules/grafana"
  prometheus_endpoints = {
    # This is obviously wrong, but it's just an example
    # and you can check on GrafanaCloud to validate the 
    # data source was created
    some = "http://example.com:1234"
  }
}