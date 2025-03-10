variable "prometheus_endpoints" {
  description = "A map of Prometheus endpoints to add as data sources."
  type        = map(string)
}

variable "dashboards" {
  description = "A map of dashboards to add."
  type        = map(string)
  default     = {}
}
