variable "dashboards" {
  description = "A map of dashboards to add."
  type        = map(string)
  default     = {}
}

variable "cluster_reference" {
  description = "The cluster reference to use for the Grafana data source."
  type        = string
}

variable "hosted_grafana_id" {
  description = "The hosted Grafana ID for the Grafana PDC agent."
  type        = string
}

variable "grafana_region" {
  description = "The region for the Grafana instance."
  type        = string
}
