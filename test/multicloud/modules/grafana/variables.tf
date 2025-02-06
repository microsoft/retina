variable "prometheus_endpoints" {
  description = "A map of Prometheus endpoints to add as data sources."
  type        = map(string)
  default = {
    aks  = "http://85.210.188.53:9090"
    kind = "http://127.0.0.1:9090"
  }
}
