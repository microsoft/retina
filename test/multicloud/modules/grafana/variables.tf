variable "prometheus_endpoints" {
  description = "A map of Prometheus endpoints to add as data sources."
  type        = map(string)
}
