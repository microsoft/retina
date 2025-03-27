variable "project" {
  description = "Project ID"
  type        = string
}

variable "grafana_url" {
  description = "The URL of the Grafana instance"
  type        = string
}

variable "grafana_cloud_access_policy_token" {
  description = "The Cloud Access Policy token required for Grafana Cloud API operations"
  type        = string
  sensitive   = true
}

variable "grafana_pdc_hosted_grafana_id" {
  description = "The hosted Grafana ID for the Grafana PDC agent."
  type        = string
}

variable "grafana_pdc_cluster" {
  description = "The cluster name for the Grafana PDC agent."
  type        = string
}