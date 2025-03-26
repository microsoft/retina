variable "dashboards" {
  description = "A map of dashboards to add."
  type        = map(string)
  default     = {}
}

variable "cluster_reference" {
  description = "The cluster reference to use for the Grafana data source."
  type        = string
}
